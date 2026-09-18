package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/hebernet/hebernet/internal/protocol"
)

type Server struct {
	SocketPath string
	DataDir    string
	DryRun     bool
	mu         sync.Mutex
}

func (s *Server) ListenAndServe() error {
	if err := os.MkdirAll(filepath.Dir(s.SocketPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(s.SocketPath)
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.SocketPath, 0o660); err != nil {
		log.Printf("chmod socket: %v", err)
	}
	// API runs as group hebernet — directory + socket must be group-reachable.
	secureSocketForAPI(s.SocketPath)
	log.Printf("agent listening on %s (dry-run=%v)", s.SocketPath, s.DryRun)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	var req protocol.Request
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(protocol.Response{OK: false, Error: "invalid request"})
		return
	}
	res := s.dispatch(req)
	_ = enc.Encode(res)
}

func (s *Server) dispatch(req protocol.Request) protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := protocol.Response{ID: req.ID, Result: map[string]any{}}
	var err error
	switch req.Op {
	case protocol.OpPing:
		res.Result["pong"] = true
		res.Result["dry_run"] = s.DryRun
	case protocol.OpCreateSiteUser:
		err = s.createSiteUser(req.Params, res.Result)
	case protocol.OpDeleteSiteUser:
		err = s.deleteSiteUser(req.Params, res.Result)
	case protocol.OpDestroySite:
		err = s.destroySite(req.Params, res.Result)
	case protocol.OpSetQuota:
		err = s.setQuota(req.Params, res.Result)
	case protocol.OpProvisionNginx:
		err = s.provisionNginx(req.Params, res.Result)
	case protocol.OpProvisionPHP:
		err = s.provisionPHP(req.Params, res.Result)
	case protocol.OpProvisionWP:
		err = s.provisionWordPress(req.Params, res.Result)
	case protocol.OpProvisionLaravel:
		err = s.provisionLaravel(req.Params, res.Result)
	case protocol.OpProvisionPrestaShop:
		err = s.provisionPrestaShop(req.Params, res.Result)
	case protocol.OpIssueSSL:
		err = s.issueSSL(req.Params, res.Result)
	case protocol.OpSuspendSite:
		err = s.suspendSite(req.Params, true, res.Result)
	case protocol.OpResumeSite:
		err = s.suspendSite(req.Params, false, res.Result)
	case protocol.OpResetSFTPPass:
		err = s.resetSFTP(req.Params, res.Result)
	case protocol.OpCreateDatabase:
		err = s.createDatabase(req.Params, res.Result)
	case protocol.OpQuotaUsage:
		err = s.quotaUsage(req.Params, res.Result)
	case protocol.OpReadLogs:
		err = s.readLogs(req.Params, res.Result)
	case protocol.OpResetDBPass:
		err = s.resetDBPass(req.Params, res.Result)
	case protocol.OpSyncCron:
		err = s.syncCron(req.Params, res.Result)
	case protocol.OpInstallAdminer:
		err = s.installAdminer(req.Params, res.Result)
	case protocol.OpEnsureFM:
		err = s.ensureFilebrowser(req.Params, res.Result)
	default:
		err = fmt.Errorf("unknown op %q", req.Op)
	}
	if err != nil {
		res.OK = false
		res.Error = err.Error()
		return res
	}
	res.OK = true
	return res
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func num(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// secureSocketForAPI sets dir+socket to root:hebernet so the API user can connect.
func secureSocketForAPI(socketPath string) {
	dir := filepath.Dir(socketPath)
	_ = os.Chmod(dir, 0o750)
	g, err := user.LookupGroup("hebernet")
	if err != nil {
		log.Printf("lookup group hebernet: %v (socket may be API-inaccessible)", err)
		return
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return
	}
	if err := os.Chown(dir, 0, gid); err != nil {
		log.Printf("chown %s: %v", dir, err)
	}
	if err := os.Chown(socketPath, 0, gid); err != nil {
		log.Printf("chown %s: %v", socketPath, err)
	}
}
