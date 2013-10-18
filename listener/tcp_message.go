package listener

import (
	"github.com/akrennmair/gopcap"
	"sort"
	"time"
)

const MSG_EXPIRE = 200 * time.Millisecond

// TCPMessage ensure that all TCP packets for given request is received, and processed in right sequence
// Its needed because all TCP message can be fragmented or re-transmitted
//
// Each TCP Packet have 2 ids: acknowledgment - message_id, and sequence - packet_id
// Message can be compiled from unique packets with same message_id which sorted by sequence
// Message is received if we didn't receive any packets for 200ms
type TCPMessage struct {
	Ack     uint32 // Message ID
	Key     [4]uint32
	packets []*pcap.Packet

	timer *time.Timer // Used for expire check

	c_packets chan *pcap.Packet

	c_del_message chan *TCPMessage
}

// NewTCPMessage pointer created from a Acknowledgment number and a channel of messages readuy to be deleted
func NewTCPMessage(Ack uint32, Key [4]uint32, c_del chan *TCPMessage) (msg *TCPMessage) {
	msg = &TCPMessage{Ack: Ack, Key: Key}

	msg.c_packets = make(chan *pcap.Packet)
	msg.c_del_message = c_del // used for notifying that message completed or expired

	// Every time we receive packet we reset this timer
	msg.timer = time.AfterFunc(MSG_EXPIRE, msg.Timeout)

	go msg.listen()

	return
}

func (t *TCPMessage) listen() {
	for {
		select {
		case packet, more := <-t.c_packets:
			if more {
				t.AddPacket(packet)
			} else {
				// Stop loop if channel closed
				return
			}
		}
	}
}

// Timeout notifies message to stop listening, close channel and message ready to be sent
func (t *TCPMessage) Timeout() {
	c := t.c_packets
	if c == nil {
		return
	}
	t.c_packets = nil
	close(c)   // Notify to stop listen loop and close channel
	t.c_del_message <- t // Notify RAWListener that message is ready to be send to replay server
}

type SortablePackets []*pcap.Packet

func (s SortablePackets) Len() int      { return len(s) }
func (s SortablePackets) Swap(i, j int) { s[i], s[j] = s[j], s[i] }


func (s SortablePackets) Less(i, j int) bool {
	seq_i := s[i].Headers[1].(*pcap.Tcphdr).Seq
	seq_j := s[j].Headers[1].(*pcap.Tcphdr).Seq
	return seq_i < seq_j
}

// Bytes sorts packets in right orders and return message content
func (t *TCPMessage) Bytes() (output []byte) {
	/*mk := make([]int, len(t.packets))*/

	/*i := 0*/
	/*for k, p := range t.packets {*/
		/*seq := int(p.Headers[1].(*pcap.Tcphdr).Seq)*/
		/*mk[i] = seq*/
		/*i++*/
	/*}*/

	/*sort.Ints(mk)*/

	sort.Sort(SortablePackets(t.packets))

	for _, p := range t.packets {
		output = append(output, p.Payload...)
	}

	return
}

// AddPacket to the message and ensure packet uniqueness
// TCP allows that packet can be re-send multiple times
func (t *TCPMessage) AddPacket(packet *pcap.Packet) {
	packetFound := false
	seq := int(packet.Headers[1].(*pcap.Tcphdr).Seq)

	for _, pkt := range t.packets {
		pkt_seq := int(pkt.Headers[1].(*pcap.Tcphdr).Seq)
		if seq == pkt_seq {
			packetFound = true
			break
		}
	}

	if packetFound {
		Debug("Received packet with same sequence")
	} else {
		t.packets = append(t.packets, packet)
	}

	// Reset message timeout timer
	t.timer.Reset(MSG_EXPIRE)
}
