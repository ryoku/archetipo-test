import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createUserManager } from "./oidc";

const { userManagerCtor, stateStoreCtor } = vi.hoisted(() => ({
  userManagerCtor: vi.fn(),
  stateStoreCtor: vi.fn(),
}));

vi.mock("oidc-client-ts", () => ({
  UserManager: userManagerCtor,
  WebStorageStateStore: stateStoreCtor,
}));

describe("createUserManager", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv("VITE_OIDC_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("VITE_OIDC_CLIENT_ID", "kubegate-frontend");
    stateStoreCtor.mockImplementation(function ({ store }) {
      return { store };
    });
    userManagerCtor.mockImplementation(function (config) {
      return { config };
    });
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("creates a user manager with the configured OIDC URLs and session storage", () => {
    const manager = createUserManager();

    expect(stateStoreCtor).toHaveBeenCalledWith({
      store: globalThis.sessionStorage,
    });
    expect(userManagerCtor).toHaveBeenCalledWith({
      authority: "https://issuer.example.com",
      client_id: "kubegate-frontend",
      redirect_uri: `${globalThis.location.origin}/auth/callback`,
      post_logout_redirect_uri: `${globalThis.location.origin}/login`,
      scope: "openid profile email",
      userStore: { store: globalThis.sessionStorage },
    });
    expect(manager).toEqual({
      config: expect.objectContaining({
        authority: "https://issuer.example.com",
        client_id: "kubegate-frontend",
      }),
    });
  });
});
