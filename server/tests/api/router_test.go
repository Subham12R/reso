package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/subham12r/reso/internal/api"
	"github.com/subham12r/reso/internal/rooms"
)

func TestRouterRoutesRoomCreation(t *testing.T) {
	router := api.NewRouter(rooms.NewRoomService())

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms",
		bytes.NewBufferString(`{"displayName":"Subham"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestRouterRoutesJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	router := api.NewRouter(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/join-requests",
		bytes.NewBufferString(
			`{"code":"`+created.Code+`","displayName":"Alex"}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestRouterRoutesJoinApproval(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joinRequest, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	router := api.NewRouter(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/"+created.Room.ID+
			"/join-requests/"+joinRequest.ID+"/approve",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRouterRoutesJoinRejection(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joinRequest, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	router := api.NewRouter(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/"+created.Room.ID+
			"/join-requests/"+joinRequest.ID+"/reject",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRouterRoutesPendingJoinRequestList(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	router := api.NewRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/rooms/"+created.Room.ID+"/join-requests",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
