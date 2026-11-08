// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

let bar:HTMLDivElement|null=null;
export default{configure(){},start(){if(bar)return;bar=document.createElement('div');bar.setAttribute('role','progressbar');bar.style.cssText='position:fixed;top:0;left:0;width:75%;height:3px;background:var(--el-color-primary,#2563eb);z-index:99999;transition:width .2s';document.body.append(bar);},done(){const current=bar;bar=null;if(current){current.style.width='100%';setTimeout(()=>current.remove(),200);}}};
