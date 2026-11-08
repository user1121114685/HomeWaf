// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

function getOnlineUrl(){
  return "/#/help?topic="
}

// 将四个全局公共方法，组合成一个对象，并暴露出去
export default {
  getOnlineUrl,
}
