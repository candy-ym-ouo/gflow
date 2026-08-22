package api

import (
	"encoding/json"
	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
	"example.com/gflow/internal/validator"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Router struct {
	s store.Store
	e *engine.Engine
}

func New(s store.Store, e *engine.Engine) *Router { return &Router{s: s, e: e} }
func (r *Router) Routes() http.Handler            { return http.HandlerFunc(r.serve) }
func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func (r *Router) serve(w http.ResponseWriter, q *http.Request) {
	p := strings.TrimPrefix(q.URL.Path, "/api/v1")
	if p == "/healthz" {
		write(w, map[string]string{"status": "ok"})
		return
	}
	if p == "/readyz" {
		write(w, map[string]string{"status": "ready"})
		return
	}
	if p == "/metrics" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("gflow_up 1\n"))
		return
	}
	if p == "/instances" && q.Method == "GET" {
		r.listInstances(w, q)
		return
	}
	if p == "/approval-tasks" && q.Method == "GET" {
		write(w, r.s.ListTasks(q.URL.Query().Get("assignee")))
		return
	}
	if strings.HasPrefix(p, "/approval-tasks/") {
		r.approvalTask(w, q, strings.TrimPrefix(p, "/approval-tasks/"))
		return
	}
	if p == "/dead-letters" && q.Method == "GET" {
		write(w, r.s.ListDeadLetters())
		return
	}
	if p == "/events/stream" {
		r.stream(w, q)
		return
	}
	if p == "/workflows" && q.Method == "POST" {
		var d model.WorkflowDefinition
		if json.NewDecoder(q.Body).Decode(&d) != nil {
			http.Error(w, "bad json", 400)
			return
		}
		if d.ID == "" {
			d.ID = d.Name
		}
		if es := validator.Validate(d); len(es) > 0 {
			write(w, report{false, es})
			return
		}
		d.Status = "draft"
		r.s.SaveWorkflow(d)
		write(w, d)
		return
	}
	if p == "/workflows" && q.Method == "GET" {
		write(w, r.s.ListWorkflows())
		return
	}
	if p == "/workflows/validate" {
		var d model.WorkflowDefinition
		json.NewDecoder(q.Body).Decode(&d)
		es := validator.Validate(d)
		write(w, report{len(es) == 0, es})
		return
	}
	if strings.HasPrefix(p, "/workflows/") {
		r.workflow(w, q, strings.TrimPrefix(p, "/workflows/"))
		return
	}
	if strings.HasPrefix(p, "/instances/") {
		r.instance(w, q, strings.TrimPrefix(p, "/instances/"))
		return
	}
	http.NotFound(w, q)
}
func (r *Router) workflow(w http.ResponseWriter, q *http.Request, id string) {
	if strings.HasSuffix(id, "/publish") {
		id = strings.TrimSuffix(id, "/publish")
		d, e := r.s.GetWorkflow(id)
		if e != nil {
			http.NotFound(w, q)
			return
		}
		d.Status = "published"
		r.s.SaveWorkflow(d)
		write(w, d)
		return
	}
	if strings.HasSuffix(id, "/instances") && q.Method == "POST" {
		id = strings.TrimSuffix(id, "/instances")
		d, e := r.s.GetWorkflow(id)
		if e != nil {
			http.NotFound(w, q)
			return
		}
		var x createInstance
		json.NewDecoder(q.Body).Decode(&x)
		i, e := r.e.Start(d, x.BizKey, x.Context)
		if e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		write(w, i)
		return
	}
	d, e := r.s.GetWorkflow(id)
	if e != nil {
		http.NotFound(w, q)
		return
	}
	write(w, d)
}
func (r *Router) instance(w http.ResponseWriter, q *http.Request, id string) {
	if strings.HasSuffix(id, "/events") {
		r.instanceEvents(w, q, strings.TrimSuffix(id, "/events"))
		return
	}
	if strings.HasSuffix(id, "/nodes") {
		r.instanceNodes(w, q, strings.TrimSuffix(id, "/nodes"))
		return
	}
	if strings.HasSuffix(id, "/suspend") {
		r.instanceOperation(w, q, strings.TrimSuffix(id, "/suspend"), "suspend")
		return
	}
	if strings.HasSuffix(id, "/resume") {
		r.instanceOperation(w, q, strings.TrimSuffix(id, "/resume"), "resume")
		return
	}
	if strings.HasSuffix(id, "/cancel") {
		r.instanceOperation(w, q, strings.TrimSuffix(id, "/cancel"), "cancel")
		return
	}
	if strings.HasSuffix(id, "/retry") {
		r.instanceOperation(w, q, strings.TrimSuffix(id, "/retry"), "retry")
		return
	}
	if strings.HasSuffix(id, "/approve") {
		id = strings.TrimSuffix(id, "/approve")
		var x approvalReq
		json.NewDecoder(q.Body).Decode(&x)
		if e := r.e.Approve(id, x.Actor); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		write(w, map[string]string{"status": "ok"})
		return
	}
	i, e := r.s.GetInstance(id)
	if e != nil {
		http.NotFound(w, q)
		return
	}
	write(w, response{Instance: i})
}

func (r *Router) listInstances(w http.ResponseWriter, q *http.Request) {
	page, size := positiveInt(q.URL.Query().Get("page"), 1), positiveInt(q.URL.Query().Get("pageSize"), 20)
	if size > 100 {
		size = 100
	}
	filter := store.InstanceFilter{WorkflowID: q.URL.Query().Get("workflowId"), Status: q.URL.Query().Get("status"), BizKey: q.URL.Query().Get("bizKey"), IncludeTerminal: true, Limit: size, Offset: (page - 1) * size}
	items := r.s.ListInstances(filter)
	filter.Limit, filter.Offset = 0, 0
	write(w, listResponse{Items: items, Total: r.s.CountInstances(filter), Page: page, PageSize: size})
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func (r *Router) instanceNodes(w http.ResponseWriter, q *http.Request, id string) {
	if _, err := r.s.GetInstance(id); err != nil {
		http.NotFound(w, q)
		return
	}
	write(w, r.s.ListNodes(id))
}
func (r *Router) instanceEvents(w http.ResponseWriter, q *http.Request, id string) {
	if _, err := r.s.GetInstance(id); err != nil {
		http.NotFound(w, q)
		return
	}
	cursor, _ := strconv.ParseInt(q.URL.Query().Get("cursor"), 10, 64)
	events := r.s.EventsAfter(id, cursor, positiveInt(q.URL.Query().Get("limit"), 100))
	var next int64
	if len(events) > 0 {
		next = events[len(events)-1].Seq
	}
	write(w, eventResponse{Events: events, NextCursor: next})
}

func (r *Router) approvalTask(w http.ResponseWriter, q *http.Request, id string) {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	if len(parts) == 1 && q.Method == "GET" {
		task, err := r.s.GetTask(parts[0])
		if err != nil {
			http.NotFound(w, q)
			return
		}
		write(w, task)
		return
	}
	if len(parts) != 2 || q.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req approvalReq
	_ = json.NewDecoder(q.Body).Decode(&req)
	var err error
	switch parts[1] {
	case "approve":
		err = r.e.Approve(parts[0], req.Actor)
	case "reject":
		err = r.e.Reject(parts[0], req.Actor)
	default:
		http.NotFound(w, q)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	task, _ := r.s.GetTask(parts[0])
	write(w, task)
}

func (r *Router) instanceOperation(w http.ResponseWriter, q *http.Request, id, operation string) {
	if q.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	i, err := r.s.GetInstance(id)
	if err != nil {
		http.NotFound(w, q)
		return
	}
	var req operationRequest
	_ = json.NewDecoder(q.Body).Decode(&req)
	switch operation {
	case "suspend":
		if i.Status != model.Suspended {
			i.Status = model.Suspended
		}
	case "resume":
		if i.Status == model.Suspended {
			i.Status = model.Running
		}
	case "cancel":
		i.Status = model.Terminated
		now := time.Now()
		i.CompletedAt = &now
	case "retry":
		i.Status = model.Running
		i.ErrorInfo = ""
		i.NextRetryAt = nil
	}
	if err := r.s.UpdateInstance(i); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	r.s.AppendEvent(model.NewEvent(id, "", "INSTANCE_"+strings.ToUpper(operation), req.Actor, nil))
	write(w, response{Instance: i})
}

func (r *Router) stream(w http.ResponseWriter, q *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, cancel := r.s.Subscribe()
	defer cancel()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", event.Seq, data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-q.Context().Done():
			return
		}
	}
}
