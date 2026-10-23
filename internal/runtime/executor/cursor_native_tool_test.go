package executor

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/auth/cursor/proto"
	"github.com/tidwall/gjson"
)

func TestNativeClientToolMapsReadWriteGrep(t *testing.T) {
	before := int64(2)
	insensitive := true
	readName, readArgs, ok := nativeClientTool(&proto.DecodedServerMessage{
		Type: proto.ServerMsgExecReadArgs,
		Path: "/etc/hostname",
	})
	if !ok || readName != "Read" || gjson.Get(readArgs, "file_path").String() != "/etc/hostname" {
		t.Fatalf("read = %s %s %v", readName, readArgs, ok)
	}

	writeName, writeArgs, ok := nativeClientTool(&proto.DecodedServerMessage{
		Type:     proto.ServerMsgExecWriteArgs,
		Path:     "/tmp/out.txt",
		FileText: "hello",
	})
	if !ok || writeName != "Write" || gjson.Get(writeArgs, "content").String() != "hello" {
		t.Fatalf("write = %s %s %v", writeName, writeArgs, ok)
	}

	grepName, grepArgs, ok := nativeClientTool(&proto.DecodedServerMessage{
		Type:            proto.ServerMsgExecGrepArgs,
		Pattern:         "HELLO",
		Path:            "/etc",
		Glob:            "*.txt",
		ContextBefore:   &before,
		CaseInsensitive: &insensitive,
	})
	if !ok || grepName != "Grep" || gjson.Get(grepArgs, "pattern").String() != "HELLO" || gjson.Get(grepArgs, "-B").Int() != 2 || !gjson.Get(grepArgs, "-i").Bool() {
		t.Fatalf("grep = %s %s %v", grepName, grepArgs, ok)
	}

	if _, _, ok := nativeClientTool(&proto.DecodedServerMessage{Type: proto.ServerMsgExecGrepArgs}); ok {
		t.Fatal("empty grep pattern was forwarded")
	}
}

func TestNativePendingKeepsH2ParkedForClaude(t *testing.T) {
	pending, ok := nativePending(&proto.DecodedServerMessage{
		Type:     proto.ServerMsgExecGrepArgs,
		Pattern:  "HELLO",
		Path:     "/etc",
		ExecId:   "grep-1",
		ExecMsgId: 9,
	})
	if !ok || pending.SkipResume || pending.ToolName != "Grep" || pending.ExecId != "grep-1" {
		t.Fatalf("pending = %+v ok=%v", pending, ok)
	}
	reply := encodeNativeExecResult(pending, nil, "/etc/hosts:1:HELLO")
	if len(reply) == 0 {
		t.Fatal("empty grep success reply")
	}
}
