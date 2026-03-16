import { config } from "../config";

interface FirebaseSignInResponse {
  idToken: string;
  refreshToken: string;
  expiresIn: string;
  localId: string;
  email: string;
}

export async function signInWithFirebaseRest(): Promise<FirebaseSignInResponse> {
  const response = await fetch(
    `https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=${config.firebaseApiKey}`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email: config.firebaseTestUserEmail,
        password: config.firebaseTestUserPassword,
        returnSecureToken: true,
      }),
    },
  );

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Firebase sign-in failed: ${response.status} ${body}`);
  }

  const data = (await response.json()) as FirebaseSignInResponse;
  if (data.localId !== config.firebaseUUID) {
    throw new Error(`Unexpected Firebase localId ${data.localId}, expected ${config.firebaseUUID}`);
  }
  return data;
}
