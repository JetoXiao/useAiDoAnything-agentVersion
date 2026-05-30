CREATE TABLE IF NOT EXISTS affiliate_agent_levels (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    rebate_rate_percent DECIMAL(5,2) NOT NULL,
    min_invited_count INTEGER NOT NULL DEFAULT 0,
    min_history_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_agent_levels_rate CHECK (rebate_rate_percent >= 0 AND rebate_rate_percent <= 100),
    CONSTRAINT chk_affiliate_agent_levels_invited CHECK (min_invited_count >= 0),
    CONSTRAINT chk_affiliate_agent_levels_history CHECK (min_history_quota >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_affiliate_agent_levels_default
    ON affiliate_agent_levels (is_default)
    WHERE is_default = true;

CREATE INDEX IF NOT EXISTS idx_affiliate_agent_levels_enabled_rank
    ON affiliate_agent_levels (enabled, sort_order, min_invited_count, min_history_quota);

INSERT INTO affiliate_agent_levels (
    code,
    name,
    rebate_rate_percent,
    min_invited_count,
    min_history_quota,
    sort_order,
    enabled,
    is_default,
    created_at,
    updated_at
)
VALUES
    ('BRONZE', '青铜代理', 10, 0, 0, 10, true, true, NOW(), NOW()),
    ('SILVER', '白银代理', 20, 10, 100, 20, true, false, NOW(), NOW()),
    ('GOLD', '黄金代理', 30, 30, 500, 30, true, false, NOW(), NOW())
ON CONFLICT (code) DO NOTHING;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS aff_level_id BIGINT NULL REFERENCES affiliate_agent_levels(id) ON DELETE SET NULL;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS aff_level_manual BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_aff_level_id
    ON user_affiliates (aff_level_id)
    WHERE aff_level_id IS NOT NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rebate_rate_percent DECIMAL(5,2) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS agent_level_id BIGINT NULL REFERENCES affiliate_agent_levels(id) ON DELETE SET NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS agent_level_code VARCHAR(32) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS agent_level_name VARCHAR(64) NULL;

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_agent_level_id
    ON user_affiliate_ledger (agent_level_id)
    WHERE agent_level_id IS NOT NULL;

COMMENT ON TABLE affiliate_agent_levels IS '代理等级配置：后台可配置返佣比例与递进门槛';
COMMENT ON COLUMN affiliate_agent_levels.rebate_rate_percent IS '该代理等级的返佣比例（百分比 0-100）';
COMMENT ON COLUMN affiliate_agent_levels.min_invited_count IS '自动晋级到该等级所需的最少邀请人数';
COMMENT ON COLUMN affiliate_agent_levels.min_history_quota IS '自动晋级到该等级所需的历史返佣金额';
COMMENT ON COLUMN affiliate_agent_levels.sort_order IS '等级排序，数值越大等级越高';
COMMENT ON COLUMN user_affiliates.aff_level_id IS '手动指定的代理等级；NULL 表示按递进门槛自动匹配';
COMMENT ON COLUMN user_affiliates.aff_level_manual IS '是否由管理员手动指定代理等级';
COMMENT ON COLUMN user_affiliate_ledger.rebate_rate_percent IS '产生该笔返佣时实际使用的返佣比例快照';
COMMENT ON COLUMN user_affiliate_ledger.agent_level_id IS '产生该笔返佣时使用的代理等级 ID 快照';
COMMENT ON COLUMN user_affiliate_ledger.agent_level_code IS '产生该笔返佣时使用的代理等级编码快照';
COMMENT ON COLUMN user_affiliate_ledger.agent_level_name IS '产生该笔返佣时使用的代理等级名称快照';
