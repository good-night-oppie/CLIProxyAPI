package proto

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func wireBytesForTest(field protowire.Number, data []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, field, protowire.BytesType), data)
}

func wireValuesForTest(t *testing.T, data []byte, wanted protowire.Number) [][]byte {
	t.Helper()
	var values [][]byte
	for len(data) > 0 {
		field, kind, n := protowire.ConsumeTag(data)
		if n < 0 {
			t.Fatal("invalid tag")
		}
		data = data[n:]
		if kind == protowire.BytesType {
			value, n := protowire.ConsumeBytes(data)
			if n < 0 {
				t.Fatal("invalid bytes")
			}
			data = data[n:]
			if field == wanted {
				values = append(values, value)
			}
		} else {
			n := protowire.ConsumeFieldValue(field, kind, data)
			if n < 0 {
				t.Fatal("invalid field")
			}
			data = data[n:]
		}
	}
	return values
}

func TestMcpStateRequestAndRegisteredToolReply(t *testing.T) {
	exec := wireBytesForTest(19, []byte("span"))
	exec = append(exec, wireBytesForTest(36, wireBytesForTest(1, []byte("proxy")))...)
	message, err := DecodeAgentServerMessage(wireBytesForTest(ASM_ExecServerMessage, exec))
	if err != nil || message.Type != ServerMsgExecMcpState || len(message.McpServerIdentifiers) != 1 || message.McpServerIdentifiers[0] != "proxy" {
		t.Fatalf("MCP state request = %+v, %v", message, err)
	}
	tools := []McpToolDef{{Name: "echo", Description: "Echo input", InputSchema: []byte(`{"type":"object","properties":{"text":{"type":"string"}}}`)}}
	reply := EncodeExecMcpStateResult(7, "exec-7", message.McpServerIdentifiers, tools)
	client := wireValuesForTest(t, reply, ACM_ExecClientMessage)[0]
	if string(wireValuesForTest(t, client, ECM_ExecId)[0]) != "exec-7" {
		t.Fatal("lost exec id")
	}
	result := wireValuesForTest(t, client, 36)[0]
	success := wireValuesForTest(t, result, 1)[0]
	server := wireValuesForTest(t, success, 1)[0]
	if string(wireValuesForTest(t, server, 2)[0]) != "proxy" {
		t.Fatal("wrong server")
	}
	definitions := wireValuesForTest(t, server, 5)
	if len(definitions) != 1 {
		t.Fatal("unexpected tool count")
	}
	tool := newMsg("McpToolDefinition")
	if err := proto.Unmarshal(definitions[0], tool); err != nil {
		t.Fatal(err)
	}
	if tool.Get(field(tool, "tool_name")).String() != "echo" || tool.Get(field(tool, "provider_identifier")).String() != "proxy" {
		t.Fatal("lost tool identity")
	}
	if !tool.Has(field(tool, "input_schema")) {
		t.Fatal("lost tool schema")
	}

	// A server not configured by the caller must not acquire proxy tools.
	other := EncodeExecMcpStateResult(8, "other", []string{"unregistered"}, tools)
	client = wireValuesForTest(t, other, ACM_ExecClientMessage)[0]
	result = wireValuesForTest(t, client, 36)[0]
	success = wireValuesForTest(t, result, 1)[0]
	if len(wireValuesForTest(t, success, 1)) != 0 {
		t.Fatal("advertised unregistered server")
	}
}
