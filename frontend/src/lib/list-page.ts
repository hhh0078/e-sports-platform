/** 文件作用：合并列表追加页并按稳定ID去重，原始分页偏移由调用方按服务端条数推进。 */
/** 合并后保留既有顺序，同ID新记录更新展示；不使用去重后的数量推算下一页offset。 */
export function appendUniqueRows<T extends {id:string}>(existing:readonly T[],page:readonly T[]):T[]{const rows=new Map(existing.map(item=>[item.id,item]));for(const item of page)rows.set(item.id,item);return [...rows.values()]}
