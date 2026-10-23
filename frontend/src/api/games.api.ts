/** 文件作用：封装平台初始化和游戏目录接口。 */
import { requestPublic } from './client'
import type { Bootstrap, Game } from '../types/models'
/** 读取平台主题、公告和开发状态。 */
export function getBootstrap(): Promise<Bootstrap> { return requestPublic('/bootstrap') }
/** 读取上架游戏。 */
export function listGames(): Promise<Game[]> { return requestPublic('/games') }
