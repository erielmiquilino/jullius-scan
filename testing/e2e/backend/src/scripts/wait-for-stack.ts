import { setTimeout as sleep } from "node:timers/promises";
import { Client } from "pg";
import Redis from "ioredis";
import { config } from "../config";

async function waitFor(url: string, timeoutMs: number): Promise<void> {
  const start = Date.now();
  let lastError: unknown;

  while (Date.now() - start < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
      lastError = new Error(`Unexpected status ${response.status} from ${url}`);
    } catch (error) {
      lastError = error;
    }
    await sleep(1000);
  }

  throw new Error(`Timed out waiting for ${url}: ${String(lastError)}`);
}

async function waitForPostgres(timeoutMs: number): Promise<void> {
  const start = Date.now();
  let lastError: unknown;

  while (Date.now() - start < timeoutMs) {
    const client = new Client({ connectionString: config.pgUrl });
    try {
      await client.connect();
      await client.query("SELECT 1");
      await client.end();
      return;
    } catch (error) {
      lastError = error;
      try {
        await client.end();
      } catch {
        // ignore close errors during readiness polling
      }
    }
    await sleep(1000);
  }

  throw new Error(`Timed out waiting for PostgreSQL: ${String(lastError)}`);
}

async function waitForRedis(timeoutMs: number): Promise<void> {
  const start = Date.now();
  let lastError: unknown;

  while (Date.now() - start < timeoutMs) {
    const client = new Redis(config.redisUrl, { maxRetriesPerRequest: 1 });
    try {
      const result = await client.ping();
      client.disconnect();
      if (result === "PONG") {
        return;
      }
      lastError = new Error(`Unexpected Redis ping result ${result}`);
    } catch (error) {
      lastError = error;
      client.disconnect();
    }
    await sleep(1000);
  }

  throw new Error(`Timed out waiting for Redis: ${String(lastError)}`);
}

export async function waitForStack(): Promise<void> {
  await waitFor(`${config.baseUrl}/health`, 60_000);
  await waitFor(config.sefazMockUrl, 60_000);
  await waitForPostgres(60_000);
  await waitForRedis(60_000);
}

if (require.main === module) {
  waitForStack().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
