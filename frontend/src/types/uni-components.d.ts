/** 文件作用：声明 uni-app 原生开关事件，供复用官方控件的类型包装器统一使用。 */
/** uni-app 官方开关 change 事件载荷。 */
export interface SwitchChangeEvent { type?: string; detail: { value: boolean }; target?: object; currentTarget?: object; timeStamp?: number }
