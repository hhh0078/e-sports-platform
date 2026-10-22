// Package app 建立财务幂等索引、非负余额约束及不可变账本保护，不把历史订单状态折算为可提资金。
package app

import "context"

// migrateWallet 在启动迁移事务内补充约束；只建结构，不虚构历史充值或历史服务收入。
func migrateWallet(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `
-- 钱包所有分区金额必须为非负整数分，禁止超支或借负余额补账。
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='wallets_nonnegative' AND conrelid='wallets'::regclass) THEN
  ALTER TABLE wallets ADD CONSTRAINT wallets_nonnegative CHECK (
   COALESCE((data->>'recharge_cents')::bigint,0)>=0 AND COALESCE((data->>'available_cents')::bigint,0)>=0 AND
   COALESCE((data->>'pending_cents')::bigint,0)>=0 AND COALESCE((data->>'withdrawal_frozen_cents')::bigint,0)>=0 AND
   COALESCE((data->>'deposit_cents')::bigint,0)>=0 AND COALESCE((data->>'deposit_refunding_cents')::bigint,0)>=0);
 END IF;
END $$;
-- 每个用户请求号只能对应一份充值、提现或押金退款载荷。
CREATE UNIQUE INDEX IF NOT EXISTS wallet_topups_request_unique ON wallet_topups ((data->>'user_id'),(data->>'request_id'));
CREATE UNIQUE INDEX IF NOT EXISTS wallet_withdrawals_request_unique ON wallet_withdrawals ((data->>'user_id'),(data->>'request_id'));
CREATE UNIQUE INDEX IF NOT EXISTS wallet_deposit_refunds_request_unique ON wallet_deposit_refunds ((data->>'user_id'),(data->>'request_id'));
CREATE UNIQUE INDEX IF NOT EXISTS wallet_topups_number_unique ON wallet_topups ((data->>'order_no'));
CREATE UNIQUE INDEX IF NOT EXISTS wallet_topups_transaction_unique ON wallet_topups ((data->>'transaction_id')) WHERE COALESCE(data->>'transaction_id','')<>'';
CREATE UNIQUE INDEX IF NOT EXISTS wallet_withdrawals_bill_unique ON wallet_withdrawals ((data->>'out_bill_no'));
CREATE INDEX IF NOT EXISTS wallet_entries_user_idx ON wallet_entries ((data->>'user_id'),(data->>'created_at'));
CREATE INDEX IF NOT EXISTS wallet_escrows_provider_idx ON wallet_escrows ((data->>'provider_id'),(data->>'status'));
-- 账单只允许追加，程序错误也不能覆盖或删除已入账资金证据。
CREATE OR REPLACE FUNCTION reject_wallet_entry_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '钱包账单不可修改或删除'; END $$;
DROP TRIGGER IF EXISTS wallet_entries_immutable ON wallet_entries;
CREATE TRIGGER wallet_entries_immutable BEFORE UPDATE OR DELETE ON wallet_entries FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
COMMENT ON FUNCTION reject_wallet_entry_mutation() IS '禁止覆盖或删除钱包资金账单，财务纠正必须追加反向分录';
`)
	return err
}

// guardProductionWalletData 拒绝把开发充值和模拟托管余额带入真实资金环境，生产必须使用独立且已核账的数据源。
func guardProductionWalletData(ctx context.Context, q querier) error {
	var simulated bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_topups WHERE data->>'transaction_id' LIKE 'DEV%')
 OR EXISTS(SELECT 1 FROM wallet_escrows WHERE data->>'channel'='dev')
 OR EXISTS(SELECT 1 FROM wallet_withdrawals WHERE data->>'transfer_bill_no' LIKE 'DEV%')`).Scan(&simulated); err != nil {
		return err
	}
	if simulated {
		return fail(503, "检测到开发测试资金记录，禁止连接真实支付；请使用独立生产数据库或完成资金核账后再上线")
	}
	return nil
}
