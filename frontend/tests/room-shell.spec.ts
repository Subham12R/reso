import { expect, test } from "playwright/test";

test("chat collapses while the participant rail remains visible", async ({ page }) => {
  await page.route("**/api/v1/rooms", (route) => route.fulfill({ json: { roomId: "room-1", code: "ABC123" } }));
  await page.route("**/api/v1/rooms/room-1/state", (route) => route.fulfill({ json: { roomId: "room-1", state: "active", expiresAt: "2026-07-18T00:00:00Z" } }));
  await page.route("**/api/v1/rooms/room-1/join-requests", (route) => route.fulfill({ json: { requests: [] } }));
  await page.route("**/api/v1/rooms/room-1/media/token", (route) => route.fulfill({ status: 503 }));
  await page.goto("/");
  await page.getByLabel("Display name").fill("Owner");
  await page.getByRole("button", { name: "Create private room" }).click();
  await page.getByRole("button", { name: "Hide chat" }).click();
  await expect(page.getByText("Room Chat")).toBeHidden();
  await expect(page.getByLabel("Participant videos")).toBeVisible();
});
