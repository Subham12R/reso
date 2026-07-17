import { expect, test } from "playwright/test";

test("chat collapses while the participant rail remains visible", async ({ page }) => {
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/v1/rooms") return route.fulfill({ json: { roomId: "room-1", code: "ABC123" } });
    if (path === "/api/v1/rooms/room-1/state") return route.fulfill({ json: { roomId: "room-1", state: "active", expiresAt: "2026-07-18T00:00:00Z" } });
    if (path === "/api/v1/rooms/room-1/join-requests") return route.fulfill({ json: { requests: [] } });
    return route.fulfill({ status: 503 });
  });
  await page.goto("/");
  await page.getByLabel("Display name").fill("Owner");
  await page.getByRole("button", { name: "Create private room" }).click();
  await expect(page.getByText("Room Code: ABC123")).toBeVisible();
  await page.getByRole("button", { name: "Hide chat" }).click();
  await expect(page.getByText("Room Chat")).toBeHidden();
  await expect(page.getByLabel("Participant videos")).toBeVisible();
});
