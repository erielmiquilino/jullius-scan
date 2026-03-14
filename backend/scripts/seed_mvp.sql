-- seed_mvp.sql
-- Manual MVP provisioning: run this after migrations to set up the initial
-- House, users, and memberships for personal use.
--
-- INSTRUCTIONS:
-- 1. Create users in Firebase Auth first (via Firebase Console)
-- 2. Copy the Firebase UID for each user
-- 3. Replace the placeholder values below with real data
-- 4. Run this script against the PostgreSQL database:
--    psql $DATABASE_URL -f scripts/seed_mvp.sql

BEGIN;

-- Step 1: Create the initial House
INSERT INTO houses (name)
VALUES ('Casa Principal')
ON CONFLICT DO NOTHING;

-- Step 2: Create users (replace firebase_id and email with real values from Firebase Console)
-- User 1 (owner)
INSERT INTO users (firebase_id, email, name)
VALUES (
    'REPLACE_WITH_FIREBASE_UID_1',
    'owner@example.com',
    'Owner Name'
)
ON CONFLICT (firebase_id) DO NOTHING;

-- User 2 (member, optional — add more as needed)
-- INSERT INTO users (firebase_id, email, name)
-- VALUES (
--     'REPLACE_WITH_FIREBASE_UID_2',
--     'member@example.com',
--     'Member Name'
-- )
-- ON CONFLICT (firebase_id) DO NOTHING;

-- Step 3: Link users to the House
-- Owner membership
INSERT INTO house_members (user_id, house_id, role)
SELECT u.id, h.id, 'owner'
FROM users u, houses h
WHERE u.firebase_id = 'REPLACE_WITH_FIREBASE_UID_1'
  AND h.name = 'Casa Principal'
ON CONFLICT (user_id, house_id) DO NOTHING;

-- Member membership (uncomment if adding a second user)
-- INSERT INTO house_members (user_id, house_id, role)
-- SELECT u.id, h.id, 'member'
-- FROM users u, houses h
-- WHERE u.firebase_id = 'REPLACE_WITH_FIREBASE_UID_2'
--   AND h.name = 'Casa Principal'
-- ON CONFLICT (user_id, house_id) DO NOTHING;

COMMIT;

-- Verify provisioning
SELECT 'Users:' AS section;
SELECT id, firebase_id, email, name FROM users;

SELECT 'Houses:' AS section;
SELECT id, name FROM houses;

SELECT 'Memberships:' AS section;
SELECT hm.id, u.email, h.name AS house, hm.role
FROM house_members hm
JOIN users u ON u.id = hm.user_id
JOIN houses h ON h.id = hm.house_id;
