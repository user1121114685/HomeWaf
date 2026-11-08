// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// A single, abortable SSE request. No implicit retry of POSTs or AI requests.
export async function fetchEventSource(url:string,options:any){
  const {onopen,onmessage,onclose,onerror,openWhenHidden,...init}=options;
  try{
    const response=await fetch(url,init);await onopen?.(response);
    if(!response.ok||!response.body)throw new Error(`SSE request failed: ${response.status}`);
    const reader=response.body.getReader();const decoder=new TextDecoder();let buffer='';
    const dispatch=(frame:string)=>{
      const data:string[]=[];let event='message',id='';
      for(const line of frame.split('\n')){if(line.startsWith(':'))continue;const split=line.indexOf(':');const field=split<0?line:line.slice(0,split);let value=split<0?'':line.slice(split+1);if(value.startsWith(' '))value=value.slice(1);if(field==='data')data.push(value);else if(field==='event')event=value;else if(field==='id')id=value;}
      if(data.length)onmessage?.({data:data.join('\n'),event,id});
    };
    try{while(true){const {value,done}=await reader.read();buffer+=decoder.decode(value,{stream:!done});buffer=buffer.replace(/\r\n/g,'\n');if(buffer.length>4*1024*1024)throw new Error('SSE event exceeds size limit');let end;while((end=buffer.indexOf('\n\n'))>=0){dispatch(buffer.slice(0,end));buffer=buffer.slice(end+2);}if(done)break;}if(buffer.trim())dispatch(buffer);onclose?.();}finally{reader.releaseLock();}
  }catch(error){if(options.signal?.aborted)return;if(onerror)onerror(error);else throw error;}
}
