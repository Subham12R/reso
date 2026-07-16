package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/rooms"
)

func TestCreateRoomHandlerReturnsCodeAndSetsOwnerCookie(t *testing.T) {
	handler := handlers.NewRoomHandler(rooms.NewRoomService())

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms",
		bytes.NewBufferString(`{"displayName":"Subham"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response struct {
		RoomID string `json:"roomId"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.RoomID == "" {
		t.Fatal("room ID is empty")
	}

	if response.Code == "" {
		t.Fatal("room code is empty")
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "reso_owner_session" {
		t.Fatalf("cookie name = %q", cookie.Name)
	}

	if cookie.Value == "" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatal("owner cookie is not secure")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("same-site = %v, want Lax", cookie.SameSite)
	}
}

func TestJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	handler := handlers.NewJoinRequestHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/join-requests",
		bytes.NewBufferString(
			`{"code":"`+created.Code+`","displayName":"Alex"}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}

	var response struct {
		RequestID string `json:"requestId"`
		Status    string `json:"status"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.RequestID == "" {
		t.Fatal("request ID is empty")
	}

	if response.Status != "pending" {
		t.Fatalf("status = %q, want %q", response.Status, "pending")
	}
}

func TestApproveJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joinRequest, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	handler := handlers.NewApproveJoinRequestHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/"+created.Room.ID+"/join-requests/"+joinRequest.ID+"/approve",
		nil,
	)
	request.SetPathValue("roomId", created.Room.ID)
	request.SetPathValue("requestId", joinRequest.ID)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "reso_guest_session" {
		t.Fatalf("cookie name = %q", cookie.Name)
	}

	if cookie.Value == "" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatal("guest cookie is not secure")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("same-site = %v, want Lax", cookie.SameSite)
	}
}

func TestRejectJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joinRequest, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	handler := handlers.NewRejectJoinRequestHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rooms/"+created.Room.ID+"/join-requests/"+joinRequest.ID+"/reject",
		nil,
	)
	request.SetPathValue("roomId", created.Room.ID)
	request.SetPathValue("requestId", joinRequest.ID)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "rejected" {
		t.Fatalf("status = %q, want %q", response.Status, "rejected")
	}
}

func TestListSafeRequests(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	_, err = service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	handler := handlers.NewListPendingJoinRequestsHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/rooms/"+created.Room.ID+"/join-requests",
		nil,
	)
	request.SetPathValue("roomId", created.Room.ID)
	request.AddCookie(&http.Cookie{
		Name:  "reso_owner_session",
		Value: created.OwnerSessionToken,
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Requests []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"requests"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(response.Requests))
	}

	if response.Requests[0].Name != "Alex" {
		t.Fatalf("name = %q, want %q", response.Requests[0].Name, "Alex")
	}
}
