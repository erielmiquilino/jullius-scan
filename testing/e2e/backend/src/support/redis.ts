import Redis from "ioredis";
import { config } from "../config";

export const JOB_QUEUE_KEY = "jullius:scraping:jobs";

export function redisClient(): Redis {
  return new Redis(config.redisUrl, {
    maxRetriesPerRequest: 1,
    lazyConnect: false,
  });
}

export async function resetRedis(): Promise<void> {
  const client = redisClient();
  try {
    await client.flushdb();
  } finally {
    client.disconnect();
  }
}

export class RedisHelper {
  constructor(private readonly client: Redis) {}

  async getQueueLength(): Promise<number> {
    return this.client.llen(JOB_QUEUE_KEY);
  }

  async enqueueRaw(payload: Record<string, unknown>): Promise<number> {
    return this.client.lpush(JOB_QUEUE_KEY, JSON.stringify(payload));
  }

  async listRawQueue(): Promise<string[]> {
    return this.client.lrange(JOB_QUEUE_KEY, 0, -1);
  }
}
