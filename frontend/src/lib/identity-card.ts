/** 文件作用：在身份核验请求前校验姓名、居民身份证格式与已满十八周岁条件。 */

/** 核验表单规范化后的最小载荷，不包含接口密钥。 */
export interface IdentityInput { name:string; idcard:string; consent:true }

/**
 * 校验身份证真实日期、校验位和上海日期下的周岁，未成年不允许调用核验接口。
 * @param name - 用户输入的真实姓名。
 * @param idcard - 用户输入的十八位居民身份证号码。
 * @param consent - 是否主动同意身份信息核验。
 * @param now - 本次校验时刻，测试可注入年龄边界。
 * @returns 规范化的核验载荷；校验失败抛出供弹框显示的错误。
 */
export function validateIdentityInput(name:string,idcard:string,consent:boolean,now=new Date()):IdentityInput {
  const value=idcard.trim().toUpperCase(),realName=name.trim()
  if(Array.from(realName).length<2||Array.from(realName).length>50||/[\r\n\x00-\x1f]/.test(realName))throw new Error('请填写2—50个字的真实姓名')
  if(!/^[1-9]\d{16}[\dX]$/.test(value))throw new Error('请输入有效的18位身份证号码')
  if(value.slice(14,17)==='000')throw new Error('身份证顺序码无效，请检查后重试')
  const year=Number(value.slice(6,10)),month=Number(value.slice(10,12)),day=Number(value.slice(12,14)),birthday=new Date(Date.UTC(year,month-1,day))
  if(year<1900||birthday.getUTCFullYear()!==year||birthday.getUTCMonth()!==month-1||birthday.getUTCDate()!==day)throw new Error('身份证出生日期无效')
  const weights=[7,9,10,5,8,4,2,1,6,3,7,9,10,5,8,4,2],codes='10X98765432'
  const sum=weights.reduce((total,weight,index)=>total+Number(value[index])*weight,0)
  if(codes[sum%11]!==value[17])throw new Error('身份证号码校验失败，请检查后重试')
  const today=new Date(now.getTime()+8*60*60*1000),currentYear=today.getUTCFullYear(),currentMonth=today.getUTCMonth()+1,currentDay=today.getUTCDate()
  if(year>currentYear||year===currentYear&&(month>currentMonth||month===currentMonth&&day>currentDay))throw new Error('身份证出生日期不能晚于今天')
  const age=currentYear-year-(currentMonth<month||currentMonth===month&&currentDay<day?1:0)
  if(age<18)throw new Error('您未满18岁，我们无法为您服务，请退出该平台。')
  if(!consent)throw new Error('请先同意身份信息核验')
  return{name:realName,idcard:value,consent:true}
}
