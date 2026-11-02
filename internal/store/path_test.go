package store

import "testing"

func TestSQLiteURIWindowsDriveAndEscapedFilenames(t *testing.T) {
	for _, tc := range []struct{ path, want string }{{`C:\Users\test\history.db`, "file:///C:/Users/test/history.db"}, {`C:\Users\two words\hash#data?.db`, "file:///C:/Users/two%20words/hash%23data%3F.db"}, {"/tmp/two words/history.db", "file:///tmp/two%20words/history.db"}} {
		if got := sqliteURI(tc.path); got != tc.want {
			t.Errorf("%q: %s != %s", tc.path, got, tc.want)
		}
	}
}
