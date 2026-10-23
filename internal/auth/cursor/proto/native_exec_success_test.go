package proto

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestEncodeNativeExecSuccessReplies(t *testing.T) {
	cases := []struct {
		name   string
		reply  []byte
		execID string
		field  protowire.Number
	}{
		{"read", EncodeExecReadSuccess(1, "read-1", "/tmp/a.txt", "hello\nworld"), "read-1", ECM_ReadResult},
		{"write", EncodeExecWriteSuccess(2, "write-1", "/tmp/a.txt"), "write-1", ECM_WriteResult},
		{"delete", EncodeExecDeleteSuccess(3, "del-1", "/tmp/a.txt"), "del-1", ECM_DeleteResult},
		{"shell", EncodeExecShellSuccess(4, "sh-1", "echo hi", "/tmp", "hi\n"), "sh-1", ECM_ShellResult},
		{"fetch", EncodeExecFetchSuccess(5, "fetch-1", "https://example.com", "<html>"), "fetch-1", ECM_FetchResult},
		{"grep", EncodeExecGrepSuccess(6, "grep-1", "HELLO", "/etc", "/etc/hosts:1:HELLO"), "grep-1", ECM_GrepResult},
		{"ls", EncodeExecLsSuccess(7, "ls-1", "/tmp", "a.txt\nb.txt"), "ls-1", ECM_LsResult},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.reply) == 0 {
				t.Fatal("empty reply")
			}
			client := wireValuesForTest(t, tc.reply, ACM_ExecClientMessage)
			if len(client) != 1 {
				t.Fatalf("missing exec client message: %d", len(client))
			}
			ids := wireValuesForTest(t, client[0], ECM_ExecId)
			if len(ids) != 1 || string(ids[0]) != tc.execID {
				t.Fatalf("exec id = %q want %q", ids, tc.execID)
			}
			if len(wireValuesForTest(t, client[0], tc.field)) != 1 {
				t.Fatalf("missing result field %d", tc.field)
			}
		})
	}
}
