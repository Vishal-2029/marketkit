-- One-time repair: re-activate cancelled subscriptions that still have
-- a successful payment and a future expiry date.
--
-- Run after deploying the multiple-active-plans code change.
-- Review the SELECT preview before running the UPDATE.

-- Preview affected rows
SELECT
    s.id AS subscription_id,
    s.user_id,
    s.plan_id,
    p.name AS plan_name,
    s.status,
    s.expiry_date,
    pay.paid_at
FROM subscriptions s
JOIN payments pay
    ON pay.user_id = s.user_id
    AND pay.plan_id = s.plan_id
    AND pay.status = 'SUCCESS'
JOIN plans p ON p.id = s.plan_id
WHERE s.status = 'CANCELLED'
  AND s.expiry_date > NOW()
  AND NOT EXISTS (
      SELECT 1
      FROM subscriptions active
      WHERE active.user_id = s.user_id
        AND active.plan_id = s.plan_id
        AND active.status = 'ACTIVE'
        AND active.expiry_date > NOW()
  );

-- Re-activate cancelled subscriptions with valid expiry and successful payment
UPDATE subscriptions s
SET status = 'ACTIVE', updated_at = NOW()
FROM payments pay
WHERE pay.user_id = s.user_id
  AND pay.plan_id = s.plan_id
  AND pay.status = 'SUCCESS'
  AND s.status = 'CANCELLED'
  AND s.expiry_date > NOW()
  AND NOT EXISTS (
      SELECT 1
      FROM subscriptions active
      WHERE active.user_id = s.user_id
        AND active.plan_id = s.plan_id
        AND active.status = 'ACTIVE'
        AND active.expiry_date > NOW()
  );

-- For cancelled subs where expiry already passed but payment succeeded,
-- recreate access from payment date + plan duration (optional second pass).
INSERT INTO subscriptions (user_id, plan_id, status, start_date, expiry_date, activated_by, created_at, updated_at)
SELECT
    pay.user_id,
    pay.plan_id,
    'ACTIVE',
    COALESCE(pay.paid_at, pay.created_at),
    COALESCE(pay.paid_at, pay.created_at) + (pl.duration_days || ' days')::interval,
    'repair-script'
FROM payments pay
JOIN plans pl ON pl.id = pay.plan_id
WHERE pay.status = 'SUCCESS'
  AND NOT EXISTS (
      SELECT 1
      FROM subscriptions s
      WHERE s.user_id = pay.user_id
        AND s.plan_id = pay.plan_id
        AND s.status = 'ACTIVE'
        AND s.expiry_date > NOW()
  )
  AND COALESCE(pay.paid_at, pay.created_at) + (pl.duration_days || ' days')::interval > NOW();
