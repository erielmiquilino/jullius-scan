import { Client } from "pg";
import type Redis from "ioredis";
import { pgClient, resetDatabase, PostgresHelper } from "./postgres";
import { redisClient, resetRedis, RedisHelper } from "./redis";
import { seedBaseUserAndHouse } from "../scripts/seed";

export interface PreparedState {
  pg: Client;
  postgres: PostgresHelper;
  redis: Redis;
  redisHelper: RedisHelper;
}

export async function prepareState(): Promise<PreparedState> {
  await resetRedis();
  await resetDatabase();
  await seedBaseUserAndHouse();

  const pg = await pgClient();
  const redis = redisClient();
  return {
    pg,
    postgres: new PostgresHelper(pg),
    redis,
    redisHelper: new RedisHelper(redis),
  };
}

export async function cleanupState(state: PreparedState): Promise<void> {
  await state.pg.end();
  state.redis.disconnect();
}
