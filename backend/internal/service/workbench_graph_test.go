package service

import "testing"

func TestBuildDependencyGraphIncludesRuntimeStatusesAndFailureEdges(t *testing.T) {
	svc := NewWorkbenchService("/tmp", nil)
	snapshot := WorkbenchStackSnapshot{
		ProjectName:       "demo",
		Revision:          7,
		SourceFingerprint: "sha256:demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api"},
			{ServiceName: "cache"},
			{ServiceName: "db"},
			{ServiceName: "worker"},
		},
		Dependencies: []WorkbenchComposeDependency{
			{ServiceName: "api", DependsOn: "db"},
			{ServiceName: "api", DependsOn: "cache"},
			{ServiceName: "worker", DependsOn: "api"},
		},
	}

	graph := svc.BuildDependencyGraph(snapshot, []DockerContainer{
		{Service: "api", Status: "Up 2 minutes (healthy)"},
		{Service: "db", Status: "Exited (1) 20 seconds ago"},
		{Service: "cache", Status: "Up 10 seconds (health: starting)"},
	})

	if graph.ProjectName != "demo" || graph.Revision != 7 {
		t.Fatalf("unexpected graph metadata: %#v", graph)
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 graph nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 3 {
		t.Fatalf("expected 3 graph edges, got %d", len(graph.Edges))
	}

	statusByService := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		statusByService[node.ServiceName] = node.Status
	}
	if statusByService["api"] != workbenchGraphNodeStatusRunning {
		t.Fatalf("expected api status %q, got %q", workbenchGraphNodeStatusRunning, statusByService["api"])
	}
	if statusByService["db"] != workbenchGraphNodeStatusFailed {
		t.Fatalf("expected db status %q, got %q", workbenchGraphNodeStatusFailed, statusByService["db"])
	}
	if statusByService["cache"] != workbenchGraphNodeStatusDegraded {
		t.Fatalf("expected cache status %q, got %q", workbenchGraphNodeStatusDegraded, statusByService["cache"])
	}
	if statusByService["worker"] != workbenchGraphNodeStatusMissing {
		t.Fatalf("expected worker status %q, got %q", workbenchGraphNodeStatusMissing, statusByService["worker"])
	}

	edgeByKey := make(map[string]WorkbenchDependencyEdge, len(graph.Edges))
	for _, edge := range graph.Edges {
		edgeByKey[edge.Key] = edge
	}
	dbToAPI, ok := edgeByKey["db->api"]
	if !ok {
		t.Fatalf("missing db->api edge: %#v", graph.Edges)
	}
	if !dbToAPI.FailureSource || dbToAPI.SourceStatus != workbenchGraphNodeStatusFailed {
		t.Fatalf("expected db->api edge to be failure-sourced: %#v", dbToAPI)
	}

	cacheToAPI, ok := edgeByKey["cache->api"]
	if !ok {
		t.Fatalf("missing cache->api edge: %#v", graph.Edges)
	}
	if cacheToAPI.FailureSource {
		t.Fatalf("expected cache->api edge not to be failure-sourced: %#v", cacheToAPI)
	}

	apiToWorker, ok := edgeByKey["api->worker"]
	if !ok {
		t.Fatalf("missing api->worker edge: %#v", graph.Edges)
	}
	if apiToWorker.FailureSource {
		t.Fatalf("expected api->worker edge not to be failure-sourced: %#v", apiToWorker)
	}
}

func TestBuildDependencyGraphMarksMissingDependencySourceAsFailure(t *testing.T) {
	svc := NewWorkbenchService("/tmp", nil)
	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api"},
		},
		Dependencies: []WorkbenchComposeDependency{
			{ServiceName: "api", DependsOn: "db"},
		},
	}

	graph := svc.BuildDependencyGraph(snapshot, nil)
	if len(graph.Edges) != 1 {
		t.Fatalf("expected one edge, got %d", len(graph.Edges))
	}
	if !graph.Edges[0].FailureSource {
		t.Fatalf("expected missing dependency edge to be marked as failure source: %#v", graph.Edges[0])
	}
	if graph.Edges[0].SourceStatus != workbenchGraphNodeStatusMissing {
		t.Fatalf("expected source status %q, got %q", workbenchGraphNodeStatusMissing, graph.Edges[0].SourceStatus)
	}
}
