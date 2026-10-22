/** 文件作用：定义管理员按上海自然日统计的服务交易额，充值、押金和提现不计入成交。 */
/** 每日已核实服务付款及实际成功退款。 */
export interface AdminTradeDay { date:string; paid_cents:number; refund_cents:number; paid_orders:number; refunded_orders:number }
/** 控制台周期汇总与历史统计边界。 */
export interface AdminAnalytics { period:{start:string;end:string;timezone:'Asia/Shanghai'}; summary:{gmv_cents:number;refund_cents:number;paid_order_count:number;refunded_order_count:number}; series:AdminTradeDay[]; legacy_untracked_orders:number; dev_mode:boolean; data_mode:'local'|'production'; statistics_note:string }
