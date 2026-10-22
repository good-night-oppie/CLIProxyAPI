package proto

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestDecodeNativeReadWriteGrep(t *testing.T) {
	read := wireBytesForTest(RA_Path, []byte("/etc/hostname"))
	readMsg, err := DecodeAgentServerMessage(wireBytesForTest(ASM_ExecServerMessage, append(
		protowire.AppendVarint(protowire.AppendTag(nil, ESM_Id, protowire.VarintType), 4),
		append(wireBytesForTest(ESM_ReadArgs, read), wireBytesForTest(ESM_ExecId, []byte("read-1"))...)...,
	)))
	if err != nil || readMsg.Type != ServerMsgExecReadArgs || readMsg.Path != "/etc/hostname" || readMsg.ExecId != "read-1" {
		t.Fatalf("read = %+v, %v", readMsg, err)
	}

	write := append(wireBytesForTest(WA_Path, []byte("/tmp/out.txt")), wireBytesForTest(WA_FileText, []byte("hello"))...)
	writeMsg, err := DecodeAgentServerMessage(wireBytesForTest(ASM_ExecServerMessage, append(
		wireBytesForTest(ESM_WriteArgs, write),
		wireBytesForTest(ESM_ExecId, []byte("write-1"))...,
	)))
	if err != nil || writeMsg.Type != ServerMsgExecWriteArgs || writeMsg.Path != "/tmp/out.txt" || writeMsg.FileText != "hello" {
		t.Fatalf("write = %+v, %v", writeMsg, err)
	}

	grep := append(wireBytesForTest(GA_Pattern, []byte("HELLO")), wireBytesForTest(GA_Path, []byte("/etc"))...)
	grep = append(grep, wireBytesForTest(GA_Glob, []byte("*.txt"))...)
	grep = append(grep, protowire.AppendVarint(protowire.AppendTag(nil, GA_CaseInsensitive, protowire.VarintType), 1)...)
	grep = append(grep, protowire.AppendVarint(protowire.AppendTag(nil, GA_HeadLimit, protowire.VarintType), 5)...)
	grepMsg, err := DecodeAgentServerMessage(wireBytesForTest(ASM_ExecServerMessage, wireBytesForTest(ESM_GrepArgs, grep)))
	if err != nil || grepMsg.Type != ServerMsgExecGrepArgs || grepMsg.Pattern != "HELLO" || grepMsg.Path != "/etc" || grepMsg.Glob != "*.txt" {
		t.Fatalf("grep = %+v, %v", grepMsg, err)
	}
	if grepMsg.CaseInsensitive == nil || !*grepMsg.CaseInsensitive || grepMsg.HeadLimit == nil || *grepMsg.HeadLimit != 5 {
		t.Fatalf("grep flags = %+v", grepMsg)
	}
}
