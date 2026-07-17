package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/subham12r/ruse/internal/api/handlers"
	"github.com/subham12r/ruse/internal/rooms"
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
	if cookie.Name != "ruse_owner_session" {
		t.Fatalf("cookie name = %q", cookie.Name)
	}

	if cookie.Value == "" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatal("owner cookie is not secure")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("same-site = %v, want Lax", cookie.SameSite)
	}
}

func TestCreateRoomHandlerAllowsInsecureDevelopmentCookie(t *testing.T) {
	handler := handlers.NewRoomHandlerWithCookieSecure(rooms.NewRoomService(), false)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", bytes.NewBufferString(`{"displayName":"Subham"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if recorder.Result().Cookies()[0].Secure {
		t.Fatal("owner cookie is secure, want insecure development cookie")
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

func TestCreateRoomHandlerReportsCapacity(t *testing.T) {
	service := rooms.NewRoomService()
	for range 3 {
		if _, err := service.CreateRoom("Subham"); err != nil {
			t.Fatalf("seed room: %v", err)
		}
	}

	recorder := httptest.NewRecorder()
	handlers.NewRoomHandler(service).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/rooms", bytes.NewBufferString(`{"displayName":"Alex"}`)))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestGuestCanClaimApprovedJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joinHandler := handlers.NewJoinRequestHandler(service)
	join := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/join-requests", bytes.NewBufferString(`{"code":"`+created.Code+`","displayName":"Alex"}`))
	joinRecorder := httptest.NewRecorder()
	joinHandler.ServeHTTP(joinRecorder, join)

	var requested struct {
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(joinRecorder.Body).Decode(&requested); err != nil {
		t.Fatalf("decode join response: %v", err)
	}
	ticket := joinRecorder.Result().Cookies()[0]
	if ticket.Name != "ruse_join_ticket" || ticket.Value == "" {
		t.Fatalf("join ticket = %#v", ticket)
	}

	approve := httptest.NewRequest(http.MethodPost, "/approve", nil)
	approve.SetPathValue("roomId", created.Room.ID)
	approve.SetPathValue("requestId", requested.RequestID)
	approve.AddCookie(&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken})
	handlers.NewApproveJoinRequestHandler(service).ServeHTTP(httptest.NewRecorder(), approve)

	status := httptest.NewRequest(http.MethodGet, "/status", nil)
	status.SetPathValue("requestId", requested.RequestID)
	status.AddCookie(ticket)
	statusRecorder := httptest.NewRecorder()
	handlers.NewGuestJoinStatusHandler(service).ServeHTTP(statusRecorder, status)

	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", statusRecorder.Code, http.StatusOK)
	}
	var response struct {
		Status string `json:"status"`
		RoomID string `json:"roomId"`
	}
	if err := json.NewDecoder(statusRecorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if response.Status != "approved" || response.RoomID != created.Room.ID {
		t.Fatalf("status response = %#v", response)
	}
	for _, cookie := range statusRecorder.Result().Cookies() {
		if cookie.Name == "ruse_guest_session" && cookie.Value == ticket.Value {
			return
		}
	}
	t.Fatal("approved guest session cookie was not set")
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
		Name:  "ruse_owner_session",
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
	if cookie.Name != "ruse_guest_session" {
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
		Name:  "ruse_owner_session",
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
		Name:  "ruse_owner_session",
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
