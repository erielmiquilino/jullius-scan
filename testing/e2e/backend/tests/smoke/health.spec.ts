import { test, expect } from "../fixtures";

test("health endpoint responds with ok", async ({ api }) => {
  const response = await api.get("/health");
  expect(response.ok()).toBeTruthy();
  await expect(response.json()).resolves.toEqual({ status: "ok" });
});
