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
CREATE INDEX IF NOT EXISTS fund_lots_order_idx ON fund_lots ((data->'order_id'));
CREATE INDEX IF NOT EXISTS fund_lots_account_idx ON fund_lots ((data->>'account_id'),(data->>'bucket'));
CREATE INDEX IF NOT EXISTS fund_movements_inputs_idx ON fund_movements USING gin ((data->'inputs') jsonb_path_ops);
CREATE UNIQUE INDEX IF NOT EXISTS fund_receipt_transaction_unique ON fund_receipts ((data->>'transaction_id'));
-- 来源批次只允许在容量内消费，源元数据禁止被后续资金操作改写。
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='fund_lots_capacity' AND conrelid='fund_lots'::regclass) THEN ALTER TABLE fund_lots ADD CONSTRAINT fund_lots_capacity CHECK((data->>'amount_cents')::bigint>0 AND (data->>'remaining_cents')::bigint>=0 AND (data->>'remaining_cents')::bigint<=(data->>'amount_cents')::bigint);END IF;END $$;
CREATE OR REPLACE FUNCTION protect_fund_lot_origin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF (OLD.data-'remaining_cents') IS DISTINCT FROM (NEW.data-'remaining_cents') THEN RAISE EXCEPTION '资金来源原始信息不可改写';END IF;RETURN NEW;END $$;
DROP TRIGGER IF EXISTS fund_lot_origin_immutable ON fund_lots;
CREATE TRIGGER fund_lot_origin_immutable BEFORE UPDATE ON fund_lots FOR EACH ROW EXECUTE FUNCTION protect_fund_lot_origin();
COMMENT ON FUNCTION protect_fund_lot_origin() IS '资金批次只允许改变剩余容量，禁止改写原始来源金额与归属';
-- 账单只允许追加，程序错误也不能覆盖或删除已入账资金证据。
CREATE OR REPLACE FUNCTION reject_wallet_entry_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '钱包账单不可修改或删除'; END $$;
DROP TRIGGER IF EXISTS wallet_entries_immutable ON wallet_entries;
CREATE TRIGGER wallet_entries_immutable BEFORE UPDATE OR DELETE ON wallet_entries FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
DROP TRIGGER IF EXISTS fund_receipts_immutable ON fund_receipts;
CREATE TRIGGER fund_receipts_immutable BEFORE UPDATE OR DELETE ON fund_receipts FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
DROP TRIGGER IF EXISTS fund_movements_immutable ON fund_movements;
CREATE TRIGGER fund_movements_immutable BEFORE UPDATE OR DELETE ON fund_movements FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
DROP TRIGGER IF EXISTS fund_audits_immutable ON fund_audits;
CREATE TRIGGER fund_audits_immutable BEFORE UPDATE OR DELETE ON fund_audits FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
COMMENT ON FUNCTION reject_wallet_entry_mutation() IS '禁止覆盖或删除钱包资金账单，财务纠正必须追加反向分录';
`)
	if err != nil {
		return err
	}
	return markLegacyFundingEscrows(ctx, q)
}

// guardProductionWalletData 拒绝把开发充值和模拟托管余额带入真实资金环境，生产必须使用独立且已核账的数据源。
func guardProductionWalletData(ctx context.Context, q querier) error {
	var simulated bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_topups WHERE data->>'transaction_id' LIKE 'DEV%')
 OR EXISTS(SELECT 1 FROM fund_receipts WHERE data->>'channel'='dev')
 OR EXISTS(SELECT 1 FROM wallet_escrows WHERE data->>'channel'='dev')
 OR EXISTS(SELECT 1 FROM wallet_withdrawals WHERE data->>'transfer_bill_no' LIKE 'DEV%')`).Scan(&simulated); err != nil {
		return err
	}
	if simulated {
		return fail(503, "检测到开发测试资金记录，禁止连接真实支付；请使用独立生产数据库或完成资金核账后再上线")
	}
	return nil
}

// markLegacyFundingEscrows 仅在升级时为缺少版本字段的旧托管确定来源代际，新版明示版本不参与兼容降级。
func markLegacyFundingEscrows(ctx context.Context, q querier) error {
	for {
		records, e := list[WalletEscrow](ctx, q, "wallet_escrows", "WHERE NOT (data ? 'funding_version') ORDER BY id LIMIT 200")
		if e != nil {
			return e
		}
		if len(records) == 0 {
			return nil
		}
		for _, escrow := range records {
			evidence, e := orderFundingEvidence(ctx, q, escrow.OrderID)
			if e != nil {
				return e
			}
			version := 0
			if evidence {
				version = 1
			}
			if _, e = q.Exec(ctx, `UPDATE wallet_escrows SET data=data||jsonb_build_object('funding_version',$2::int,'legacy_source_gap',$3::boolean) WHERE id=$1 AND NOT(data ? 'funding_version')`, escrow.OrderID, version, !evidence); e != nil {
				return e
			}
		}
	}
}
