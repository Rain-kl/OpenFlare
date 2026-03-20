package service

import (
	"testing"

	"openflare/model"
)

func TestCreateProxyRouteStructuredOriginAutoCreatesOrigin(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:        "app.example.com",
		OriginScheme:  "https",
		OriginAddress: "origin.internal",
		OriginPort:    "8443",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if route.OriginID == nil || *route.OriginID == 0 {
		t.Fatal("expected route to be linked with an auto-created origin")
	}
	if route.OriginURL != "https://origin.internal:8443" {
		t.Fatalf("unexpected route origin url: %s", route.OriginURL)
	}

	origin, err := model.GetOriginByID(*route.OriginID)
	if err != nil {
		t.Fatalf("GetOriginByID failed: %v", err)
	}
	if origin.Address != "origin.internal" {
		t.Fatalf("unexpected origin address: %s", origin.Address)
	}
}

func TestUpdateOriginRewritesLinkedRouteOriginURL(t *testing.T) {
	setupServiceTestDB(t)

	origin, err := CreateOrigin(OriginInput{
		Name:    "primary-origin",
		Address: "origin-a.internal",
	})
	if err != nil {
		t.Fatalf("CreateOrigin failed: %v", err)
	}
	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:       "app.example.com",
		OriginID:     &origin.ID,
		OriginScheme: "https",
		OriginPort:   "8443",
		OriginURI:    "/api",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	updatedOrigin, err := UpdateOrigin(origin.ID, OriginInput{
		Name:    origin.Name,
		Address: "origin-c.internal",
	})
	if err != nil {
		t.Fatalf("UpdateOrigin failed: %v", err)
	}
	if updatedOrigin.Address != "origin-c.internal" {
		t.Fatalf("unexpected updated origin address: %s", updatedOrigin.Address)
	}

	reloadedRoute, err := model.GetProxyRouteByID(route.ID)
	if err != nil {
		t.Fatalf("GetProxyRouteByID failed: %v", err)
	}
	if reloadedRoute.OriginURL != "https://origin-c.internal:8443/api" {
		t.Fatalf("expected route origin url to be rewritten, got %s", reloadedRoute.OriginURL)
	}
	if reloadedRoute.Upstreams == "" || reloadedRoute.Upstreams == "[]" {
		t.Fatalf("expected route upstreams to be preserved, got %s", reloadedRoute.Upstreams)
	}
}

func TestDeleteOriginRejectsReferencedOrigin(t *testing.T) {
	setupServiceTestDB(t)

	origin, err := CreateOrigin(OriginInput{
		Address: "origin-a.internal",
	})
	if err != nil {
		t.Fatalf("CreateOrigin failed: %v", err)
	}
	if _, err = CreateProxyRoute(ProxyRouteInput{
		Domain:       "app.example.com",
		OriginID:     &origin.ID,
		OriginScheme: "https",
		OriginPort:   "443",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	if err = DeleteOrigin(origin.ID); err == nil {
		t.Fatal("expected referenced origin deletion to fail")
	}
}
