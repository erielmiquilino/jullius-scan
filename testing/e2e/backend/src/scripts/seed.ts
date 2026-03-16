import { config } from "../config";
import { pgClient } from "../support/postgres";

export async function seedBaseUserAndHouse(): Promise<void> {
  const client = await pgClient();
  try {
    await client.query("BEGIN");
    await client.query("INSERT INTO houses (id, name) VALUES (1, 'Casa Principal E2E') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name");
    await client.query(
      "INSERT INTO users (id, firebase_id, email, name) VALUES (1, $1, $2, $3) ON CONFLICT (id) DO UPDATE SET firebase_id = EXCLUDED.firebase_id, email = EXCLUDED.email, name = EXCLUDED.name",
      [config.firebaseUUID, config.firebaseTestUserEmail, "Usuario de Testes E2E"],
    );
    await client.query(
      "INSERT INTO house_members (user_id, house_id, role) VALUES (1, 1, 'owner') ON CONFLICT (user_id, house_id) DO UPDATE SET role = EXCLUDED.role",
    );
    await client.query("COMMIT");
  } catch (error) {
    await client.query("ROLLBACK");
    throw error;
  } finally {
    await client.end();
  }
}

if (require.main === module) {
  seedBaseUserAndHouse().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
