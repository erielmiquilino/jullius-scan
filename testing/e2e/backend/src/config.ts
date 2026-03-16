import path from "node:path";
import dotenv from "dotenv";

dotenv.config({ path: path.resolve(process.cwd(), ".env") });

function getEnv(name: string, fallback?: string): string {
  const value = process.env[name] || fallback;
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value;
}

export const config = {
  firebaseProjectId: getEnv("FIREBASE_PROJECT_ID", "jullius-scan"),
  firebaseApiKey: getEnv("FIREBASE_API_KEY", "jullius-scan"),
  firebaseTestUserEmail: getEnv("FIREBASE_TEST_USER_EMAIL", "usuario-de-testes@jullius.scan.com"),
  firebaseTestUserPassword: getEnv("FIREBASE_TEST_USER_PASSWORD", "usuario_de_teste"),
  firebaseUUID: getEnv("FIREBASE_UUID", "OP96R1MEAfPS5GfATCNy2c1kN6C2"),
  baseUrl: getEnv("E2E_BASE_URL", "http://localhost:18080"),
  sefazMockUrl: getEnv("E2E_SEFAZ_MOCK_URL", "http://localhost:18091/nfce-consulta-detalhada.html"),
  sefazMockInternalUrl: getEnv("E2E_SEFAZ_MOCK_INTERNAL_URL", "http://sefaz-mock:8091/nfce-consulta-detalhada.html"),
  pgUrl: getEnv("E2E_PG_URL", "postgres://jullius:jullius@localhost:15432/jullius_e2e?sslmode=disable"),
  redisUrl: getEnv("E2E_REDIS_URL", "redis://localhost:16379/0"),
};
