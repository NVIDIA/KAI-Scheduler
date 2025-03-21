package snapshot

import (
	"encoding/json"
	"net/http"

	"github.com/run-ai/kube-ai-scheduler/pkg/scheduler/framework"
	"github.com/run-ai/kube-ai-scheduler/pkg/scheduler/log"
)

type snapshotPlugin struct {
	session *framework.Session
}

func (sp *snapshotPlugin) Name() string {
	return "snapshot"
}

func (sp *snapshotPlugin) OnSessionOpen(ssn *framework.Session) {
	sp.session = ssn
	log.InfraLogger.V(3).Info("Snapshot plugin registering get-snapshot")
	ssn.AddHttpHandler("/get-snapshot", sp.serveSnapshot)
}

func (sp *snapshotPlugin) OnSessionClose(ssn *framework.Session) {
	// Handle the session close event.
}

func (sp *snapshotPlugin) serveSnapshot(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := sp.session.Cache.Snapshot()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(writer).Encode(snapshot); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func New(arguments map[string]string) framework.Plugin {
	return &snapshotPlugin{}
}
