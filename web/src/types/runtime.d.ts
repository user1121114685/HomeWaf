// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import 'vue';
import type store from '../store';
import type bus from '../bus/bus';
declare global {interface Window {vm:any;__SAMWAF_UI_CONFIG__?:Record<string,any>;__SAMWAF_SECURITY_PATH__?:string;}}
declare module 'vue' {interface ComponentCustomProperties {$store:typeof store;$bus:typeof bus;$request:import('../utils/request').ApiClient;$message:any;$notify:any;$dialog:any;$purifyHtml:(html:string)=>string;$copyText:(text:string)=>Promise<void>;samwafglobalconfig:any;$set:(object:any,key:any,value:any)=>void;$delete:(object:any,key:any)=>void;$applyFilter:(name:string,value:any,...args:any[])=>any;}}
export {};
