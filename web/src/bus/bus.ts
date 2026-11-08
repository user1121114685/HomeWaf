// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

type Handler=(...args:any[])=>void;
const listeners=new Map<string,Set<Handler>>();
const bus={
 $on(event:string,fn:Handler){if(!listeners.has(event))listeners.set(event,new Set());listeners.get(event)!.add(fn);},
 $off(event:string,fn?:Handler){if(fn)listeners.get(event)?.delete(fn);else listeners.delete(event);},
 $emit(event:string,...args:any[]){for(const fn of [...(listeners.get(event)||[])])fn(...args);},
 $once(event:string,fn:Handler){const once=(...args:any[])=>{bus.$off(event,once);fn(...args);};bus.$on(event,once);}
};export default bus;
