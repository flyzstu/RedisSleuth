package cluster

import "testing"

func TestParseNodes(t *testing.T) {
	raw := "07c37dfeb2352e0b master.example:30004@40004 master - 0 1426238317239 4 connected 10923-16383\n" +
		"1c1f slave.example:30005@40005 slave 07c37dfeb2352e0b 0 1426238318243 5 connected\n"
	nodes, err := ParseNodes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Role != "master" || nodes[0].Slots[0] != "10923-16383" {
		t.Fatalf("unexpected: %#v", nodes)
	}
	if nodes[1].Role != "replica" || nodes[1].MasterID != "07c37dfeb2352e0b" {
		t.Fatalf("unexpected replica: %#v", nodes[1])
	}
}

func TestParseClientList(t *testing.T) {
	raw := "id=3 addr=10.10.3.25:49152 name=api age=120 idle=2 flags=N db=0 qbuf=10 qbuf-free=20 obl=4 oll=2 omem=100 cmd=hgetall\n" +
		"id=4 addr=[2001:db8::1]:5555 name= age=20 idle=12 flags=N db=1 qbuf=0 qbuf-free=0 obl=0 oll=0 omem=0 cmd=ping\n"
	got, err := ParseClientList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].IP != "10.10.3.25" || got[0].Port != "49152" || got[0].InputBuf != 10 || got[0].OutputBuf != 104 {
		t.Fatalf("%#v", got[0])
	}
	if got[1].IP != "2001:db8::1" {
		t.Fatalf("%#v", got[1])
	}
}

func TestParseInfo(t *testing.T) {
	raw := "# Server\r\nredis_version:5.0.14\r\n# Stats\r\ntotal_commands_processed:42\r\n"
	got := ParseInfo(raw)
	if got["redis_version"] != "5.0.14" || got["total_commands_processed"] != "42" {
		t.Fatalf("%#v", got)
	}
}
