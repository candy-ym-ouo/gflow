package api

import "example.com/gflow/internal/model"

type createInstance struct {
	BizKey  string         `json:"bizKey"`
	Context map[string]any `json:"context"`
}
type approvalReq struct {
	Actor   string `json:"actor"`
	Opinion string `json:"opinion"`
}
type report struct {
	Valid  bool `json:"valid"`
	Errors any  `json:"errors"`
}
type response struct {
	Instance *model.WorkflowInstance `json:"instance,omitempty"`
	Error    string                  `json:"error,omitempty"`
}

type listResponse struct {
	Items    any `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}
type operationRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}
type eventResponse struct {
	Events     any   `json:"events"`
	NextCursor int64 `json:"nextCursor"`
}
