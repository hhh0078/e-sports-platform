/** 文件作用：定义后台AI兼容接口配置、秘密写入和真实连接测试的领域契约。 */
/** 配置状态只区分本地模拟、已配置AI和未配置，不把就绪误当真实连通。 */
export type AIMode='local_simulation'|'ai'|'unconfigured'
/** 仅含公开配置和密钥是否存在的后端响应，绝不含API Key。 */
export interface AISettings{base_url:string;model:string;prompt:string;has_api_key:boolean;ready:boolean;mode:AIMode;configuration_notice:string}
/** 管理员本次提交的配置；密钥为空保留已有值，不存入本机缓存。 */
export interface AISettingsInput{base_url?:string;model?:string;prompt?:string;api_key?:string;clear_api_key?:boolean}
/** 后端用固定安全文本发起真实调用的结果，不存在模拟成功模式。 */
export interface AIConnectionResult{decision:'allow'|'reject';reason:string;mode:'ai'}
