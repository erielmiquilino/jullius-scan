import { setTimeout as sleep } from "node:timers/promises";
import type { APIRequestContext } from "@playwright/test";
import type { JobResponse } from "./types";

interface PollOptions {
  timeoutMs?: number;
  intervalMs?: number;
}

export async function pollJobUntilTerminal(
  request: APIRequestContext,
  jobId: number,
  bearerToken: string,
  options: PollOptions = {},
): Promise<JobResponse> {
  const timeoutMs = options.timeoutMs ?? 30_000;
  const intervalMs = options.intervalMs ?? 1_000;
  const start = Date.now();
  let lastBody = "";

  while (Date.now() - start < timeoutMs) {
    const response = await request.get(`/api/v1/jobs/${jobId}`, {
      headers: { Authorization: `Bearer ${bearerToken}` },
    });
    lastBody = await response.text();
    if (!response.ok) {
      throw new Error(`Job polling failed with status ${response.status()}: ${lastBody}`);
    }

    const job = JSON.parse(lastBody) as JobResponse;
    if (job.status === "completed" || job.status === "failed") {
      return job;
    }

    await sleep(intervalMs);
  }

  throw new Error(`Timed out waiting for job ${jobId}. Last observed body: ${lastBody}`);
}
