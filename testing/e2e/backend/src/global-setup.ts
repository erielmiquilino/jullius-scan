import { waitForStack } from "./scripts/wait-for-stack";

export default async function globalSetup(): Promise<void> {
  await waitForStack();
}
