package snmpgo_test

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/k-sone/snmpgo"
	"github.com/k-sone/snmpgo/snmptest"
)

type receiveQueue struct {
	msg chan *snmpgo.TrapRequest
}

func (t *receiveQueue) OnTRAP(trap *snmpgo.TrapRequest) {
	t.msg <- trap
}

// takeNextTrap blocks till next trap is received
func (n *receiveQueue) takeNextTrap() *snmpgo.TrapRequest {
	limit := time.Duration(2 * time.Second)
	select {
	case m := <-n.msg:
		return m
	case <-time.After(limit):
		return nil
	}
}

func TestSendV2TrapAndReceiveIt(t *testing.T) {
	trapQueue := &receiveQueue{make(chan *snmpgo.TrapRequest)}
	s := snmptest.NewTrapServer("localhost:0", trapQueue)
	defer s.Close()

	var varBinds snmpgo.VarBinds
	oid, _ := snmpgo.NewOid("1.3.6.1.6.3.1.1.5.3")
	varBinds = append(varBinds, snmpgo.NewVarBind(snmpgo.OidSnmpTrap, oid))

	trapSender := snmptest.NewTrapSender(t, snmpgo.ListeningUDPAddress(s))
	trapSender.SendV2TrapWithBindings(true, "public", varBinds)

	trap := trapQueue.takeNextTrap()
	if trap == nil {
		t.Fatalf("trap is not received")
	}

	pdu := trap.Pdu
	if pdu.PduType() != snmpgo.SNMPTrapV2 {
		t.Fatalf("expected trapv2, got: %s", pdu.PduType())
	}

	if !reflect.DeepEqual(pdu.VarBinds(), varBinds) {
		t.Fatalf("expected pdu bindings %v, got %v", varBinds, pdu.VarBinds())
	}
}

func TestCollectMultipleTraps(t *testing.T) {
	trapQueue := &receiveQueue{make(chan *snmpgo.TrapRequest)}
	s := snmptest.NewTrapServer("localhost:0", trapQueue)
	defer s.Close()

	var varBinds snmpgo.VarBinds
	oid, _ := snmpgo.NewOid("1.3.6.1.6.3.1.1.5.3")
	varBinds = append(varBinds, snmpgo.NewVarBind(snmpgo.OidSnmpTrap, oid))

	trapSender := snmptest.NewTrapSender(t, snmpgo.ListeningUDPAddress(s))
	trapSender.SendV2TrapWithBindings(true, "public", varBinds)
	trapSender.SendV2TrapWithBindings(true, "public", varBinds)
	trapSender.SendV2TrapWithBindings(true, "public", varBinds)

	for i := 0; i < 3; i++ {
		if trapQueue.takeNextTrap() == nil {
			t.Fatalf("traps are not received at %d", i+1)
		}
	}
}

func TestSendInformRequestAndReceiveIt(t *testing.T) {
	trapQueue := &receiveQueue{make(chan *snmpgo.TrapRequest)}
	s := snmptest.NewTrapServer("localhost:0", trapQueue)
	defer s.Close()

	var varBinds snmpgo.VarBinds
	oid, _ := snmpgo.NewOid("1.3.6.1.6.3.1.1.5.3")
	varBinds = append(varBinds, snmpgo.NewVarBind(snmpgo.OidSnmpTrap, oid))

	trapSender := snmptest.NewTrapSender(t, snmpgo.ListeningUDPAddress(s))
	go trapSender.SendV2TrapWithBindings(false, "public", varBinds)

	trap := trapQueue.takeNextTrap()
	pdu := trap.Pdu

	if pdu.PduType() != snmpgo.InformRequest {
		t.Fatalf("expected inform, got: %s", pdu.PduType())
	}

	if !reflect.DeepEqual(pdu.VarBinds(), varBinds) {
		t.Fatalf("expected pdu bindings %v, got %v", varBinds, pdu.VarBinds())
	}
}

func TestSendCommunityMismatch(t *testing.T) {
	trapQueue := &receiveQueue{make(chan *snmpgo.TrapRequest)}
	s := snmptest.NewTrapServer("localhost:0", trapQueue)
	defer s.Close()

	var varBinds snmpgo.VarBinds
	oid, _ := snmpgo.NewOid("1.3.6.1.6.3.1.1.5.3")
	varBinds = append(varBinds, snmpgo.NewVarBind(snmpgo.OidSnmpTrap, oid))

	trapSender := snmptest.NewTrapSender(t, snmpgo.ListeningUDPAddress(s))
	trapSender.SendV2TrapWithBindings(true, "private", varBinds)

	trap := trapQueue.takeNextTrap()
	if trap == nil {
		t.Fatalf("trap is not received")
	}
	if trap.Error == nil {
		t.Fatalf("community validation failed")
	}
}

func TestSendBrokenPacket(t *testing.T) {
	trapQueue := &receiveQueue{make(chan *snmpgo.TrapRequest)}
	s := snmptest.NewTrapServer("localhost:0", trapQueue)
	defer s.Close()

	buf := make([]byte, 128)
	conn, err := net.Dial("udp4", snmpgo.ListeningUDPAddress(s))
	if err != nil {
		t.Fatalf("dial error %v", err)
	}
	if _, err = conn.Write(buf); err != nil {
		t.Fatalf("send packet error %v", err)
	}

	trap := trapQueue.takeNextTrap()
	if trap == nil {
		t.Fatalf("packet is not received")
	}
	if trap.Error == nil {
		t.Fatalf("packet is not broken")
	}
}
