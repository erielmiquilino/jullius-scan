import { test as base, expect, request as playwrightRequest, type APIRequestContext } from "@playwright/test";
import { config } from "../src/config";
import { cleanupState, prepareState, type PreparedState } from "../src/support/stack";
import { signInWithFirebaseRest } from "../src/support/firebase";

type BackendFixtures = {
  bearerToken: string;
  api: APIRequestContext;
  state: PreparedState;
};

export const test = base.extend<BackendFixtures>({
  bearerToken: async ({}, use) => {
    const auth = await signInWithFirebaseRest();
    await use(auth.idToken);
  },

  api: async ({}, use) => {
    const api = await playwrightRequest.newContext({
      baseURL: config.baseUrl,
      extraHTTPHeaders: { "Content-Type": "application/json" },
    });
    await use(api);
    await api.dispose();
  },

  state: async ({}, use) => {
    const state = await prepareState();
    await use(state);
    await cleanupState(state);
  },
});

export { expect };
