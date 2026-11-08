// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import proxy from '../config/host';
const env = import.meta.env.MODE || 'development';
export {AesEncrypt,AesDecrypt} from './legacyCipher';
export function copyObj(obj: any): any {
     const newObj: any =Array.isArray(obj) ? [] : {}
      for (let key in obj) {
          if (typeof obj[key] !== 'object') {
              newObj[key] = obj[key];
          } else {
              newObj[key] = copyObj(obj[key]);
          }
      }
      return newObj;
}
export function getBaseUrl(){
  const API_HOST = env === 'mock' ? '/' : proxy[env].API; // 如果是mock模式 就不配置host 会走本地Mock拦截
  return API_HOST
}

/**
 * 获取在线文档前缀
 */
export function getOnlineUrl(){
  return "/#/help?topic="
}

/**
 * 判断是否是对象
 */
export const isObject = (obj, isEffective = false) => {
  if (Object.prototype.toString.call(obj) === "[object Object]") {
    if (isEffective) {
      return !!Object.keys(obj).length;
    } else {
      return true;
    }
  } else {
    return false;
  }
};

/**
 * value 是否在list中
 * @param value
 * @param list
 */
export const isInList=(value: any, list: any[] = [])=> {
  return list.includes(value);
}

/**
 * Get default value
 * @param obj
 * @param key
 * @param defaultValue
 */
export const getOrDefault = (obj: Record<string,any> = {}, key = '', defaultValue: any = 1) => {
  //console.log("getOrDefault",obj,key,obj[key] !== undefined )
  return obj[key] !== undefined ? obj[key].toString() : defaultValue.toString();
};
