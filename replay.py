#! coding: utf-8

from gevent import monkey
monkey.patch_all()

import os
import sys
import logging
import socket
from optparse import OptionParser
import httplib

import gevent
import gevent.queue
import time

import statsd
# https://github.com/gwik/geventhttpclient
import geventhttpclient.httplib


logger = logging.getLogger()

statsd_client = None


class Counter(object):
    """ Считает число событий за последнюю секунду, умеет отправлять в statsd """
    def __init__(self, name):
        self.name = name
        self.v = 0
        self.t = int(time.time())

    def count(self):
        t = int(time.time())
        if t != self.t:
            logger.info('C %s: %d per second', self.name, self.v)
            if statsd_client:
                statsd_client.incr(self.name, self.v)
            self.on_tick()
            self.v = 1
            self.t = t
        else:
            self.v += 1

    def on_tick(self):
        pass


class Listener(object):
    """ Читает из датаграммного юникс-сокета и кладет в очередь.
        Следит за тем, чтобы очередь не переполнилась.
    """
    def __init__(self, i, unixsock, queue, options):
        self.i = i
        self.unixsock = unixsock
        self.queue = queue
        self.running = True
        self.options = options

    def next_query(self):
        pkt = self.unixsock.recv(16384)
        if len(pkt) == 0:
            logger.warning('Listener %d got empty packet, assuming disconnect', self.i)
            self.running = False
            return ''
        #pkt = pkt[6:]
        return pkt

    def runloop(self):
        i = self.i
        queue = self.queue
        incoming_requests_counter = Counter('input')

        if statsd_client:
            def on_tick():
                statsd_client.incr('backlog', queue.qsize())

            incoming_requests_counter.on_tick = on_tick

        drop_counter = Counter('dropped')
        logger.info('Listener %d started', i)
        len_limit = self.options.backlog - self.options.backlog_breathing_space

        while self.running:
            q = self.next_query()
            if not q:
                continue
            incoming_requests_counter.count()
            logger.debug('Listener %d got %s', i, q)
            if queue:
                if queue.qsize() > len_limit:
                    drop_counter.count()
                else:
                    queue.put(q)


class Worker(object):
    """ Берет запрос из очереди, быстренько парсит и отправляет в апстрим по keepalive-соединению """
    def __init__(self, i, queue, output_counter, parse_errors_counter, options):
        self.i = i
        self.queue = queue
        self.output_counter = output_counter
        self.parse_errors_counter = parse_errors_counter
        self.running = True
        self.options = options

    def connect(self):
        conn = None
        if self.options.upstream:
            host, port = self.options.upstream, 80
            if ':' in host:
                host, port = host.split(':')
                port = int(port)
            conn = geventhttpclient.httplib.HTTPConnection(host, port)
            #conn = httplib.HTTPConnection(host, port)
        return conn

    def runloop(self):
        conn = self.connect()
        i = self.i
        queue = self.queue
        output_counter = self.output_counter
        parse_errors = self.parse_errors_counter
        logger.info('Worker %d started', i)
        methodset = set(['GET', 'POST', 'PUT', 'DELETE', 'HEAD'])
        only_get = self.options.only_get

        while self.running:
            q = queue.get()
            if not q:
                continue
            output_counter.count()
            try:
                headers, body = q.split('\r\n\r\n', 1)
                method, url, _ = headers.split(' ', 2)
                if method not in methodset:
                    if method.endswith('POST'):
                        method = 'POST'
                    elif method.endswith('GET'):
                        method = 'GET'
                    else:
                        parse_errors.count()
                        continue
                if only_get and method != 'GET':
                    continue
                logger.debug([method, url])
                # TODO извлечь и сохранить X-Real-IP
            except Exception as e:
                logger.debug('error whlie parsing request: %s %s', e, q)
                parse_errors.count()
                continue
            if conn:
                try:
                    conn.request(method, url, body)
                    r = conn.getresponse()
                    r.read()
                except Exception as e:
                    logger.debug('error whlie sending request: %s %s', e, q)


def setup_options():
    parser = OptionParser()
    parser.add_option("--socket", dest="unix_socket", default="/tmp/mysock.dgram.0", action="store", help=u"path to unix seqpacket socket")
    parser.add_option("--threads", dest="threads", type=int, default=1, action="store", help=u"number of gevent threads")
    parser.add_option("--upstream", dest="upstream", default="", action="store", help=u"host:port to send HTTP requests to")

    parser.add_option("--only-GET", dest="only_get", default=False, action="store_true", help=u"forward only GET requests")

    parser.add_option("--statsd", dest="statsd", default="", action="store", help=u"host:port of statsd")

    parser.add_option("--backlog", dest="backlog", type=int, default=30000, action="store", help=u"size of backlog queue")
    parser.add_option("--backlog-breathing-space", dest="backlog_breathing_space", type=int, default=500, action="store", help=u"backlog breathing space")

    parser.add_option("--loglevel", action="store", dest="loglevel", default='INFO', choices=['DEBUG', 'INFO', 'WARNING', 'ERROR'],
                      help=u"Уровень логгирования скрипта")
    return parser.parse_args()


def get_seqpacket_socket(addr):
    unixsock = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
    unixsock.bind(addr)
    unixsock.listen(1)

    logger.info('accepting connection...')
    conn, addr = unixsock.accept()
    return conn


def get_dgram_socket(addr):
    unixsock = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
    unixsock.bind(addr)
    return unixsock


def main():
    global logger, statsd_client
    options, args = setup_options()
    logging.basicConfig(stream=sys.stdout, level=getattr(logging, options.loglevel), format="%(asctime)s :: %(message)s")
    logger = logging.getLogger()

    if os.path.exists(options.unix_socket):
        os.unlink(options.unix_socket)

    conn = get_dgram_socket(options.unix_socket)
    logger.info('spawning workers...')

    if options.statsd:
        host, port = options.statsd.split(':')
        prefix = 'gor.' + socket.gethostname().split('.')[0] + '.replay'
        statsd_client = statsd.StatsClient(host, int(port), prefix)

    total_output_counter = Counter('worker_output')
    parse_errors_counter = Counter('parse_errors')

    queue = gevent.queue.Queue(maxsize=options.backlog)

    listener = Listener(0, conn, queue, options)
    listener_thread = gevent.spawn(listener.runloop)
    workers = [Worker(i, queue, total_output_counter, parse_errors_counter, options) for i in xrange(options.threads)]
    threads = [gevent.spawn(worker.runloop) for worker in workers]
    listener_thread.join()

if __name__ == '__main__':
    main()
