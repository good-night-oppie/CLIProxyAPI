package proto

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestDecodeListMcpResourcesEmptyArgs(t *testing.T) {
	exec := protowire.AppendVarint(protowire.AppendTag(nil, ESM_Id, protowire.VarintType), 9)
	exec = append(exec, wireBytesForTest(ESM_ExecId, []byte("res-1"))...)
	exec = append(exec, wireBytesForTest(ESM_ListMcpResourcesExecArgs, nil)...)
	exec = append(exec, wireBytesForTest(19, []byte("span-context"))...)
	message, err := DecodeAgentServerMessage(wireBytesForTest(ASM_ExecServerMessage, exec))
	if err != nil {
		t.Fatal(err)
	}
	if message.Type != ServerMsgExecListMcpResources {
		t.Fatalf("type=%v want list_mcp_resources", message.Type)
	}
	if message.ExecId != "res-1" || message.ExecMsgId != 9 {
		t.Fatalf("identity = %+v", message)
	}
}

func TestEncodeListMcpResourcesEmptyReply(t *testing.T) {
	reply := EncodeExecListMcpResourcesEmpty(9, "res-1")
	client := wireValuesForTest(t, reply, ACM_ExecClientMessage)[0]
	if string(wireValuesForTest(t, client, ECM_ExecId)[0]) != "res-1" {
		t.Fatal("lost exec id")
	}
	result := wireValuesForTest(t, client, ECM_ListMcpResourcesExecResult)
	if len(result) != 1 {
		t.Fatal("missing field 17 result")
	}
	success := wireValuesForTest(t, result[0], 1)
	if len(success) != 1 {
		t.Fatal("missing success")
	}
	if len(wireValuesForTest(t, success[0], 1)) != 0 {
		t.Fatal("proxy must not invent MCP resources")
	}
}

func TestEncodeReadMcpResourceErrorReply(t *testing.T) {
	reply := EncodeExecReadMcpResourceError(4, "read-1", "no resources")
	client := wireValuesForTest(t, reply, ACM_ExecClientMessage)[0]
	if string(wireValuesForTest(t, client, ECM_ExecId)[0]) != "read-1" {
		t.Fatal("lost exec id")
	}
	if len(wireValuesForTest(t, client, ECM_ReadMcpResourceExecResult)) != 1 {
		t.Fatal("missing field 18 result")
	}
}

func TestEncodeUnknownExecEmptyReply(t *testing.T) {
	reply := EncodeExecUnknownEmpty(3, "unk-1", 21)
	client := wireValuesForTest(t, reply, ACM_ExecClientMessage)[0]
	if string(wireValuesForTest(t, client, ECM_ExecId)[0]) != "unk-1" {
		t.Fatal("lost exec id")
	}
	if len(wireValuesForTest(t, client, 21)) != 1 {
		t.Fatal("must echo unknown field number")
	}
}
