<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-list :split="true">
        <hw-list-item v-for="runtime in runtimes">
          <hw-list-item-meta :title="runtime.name" >
            <template #description="{ row }">
              <pre>{{runtime.value}}</pre>
            </template>
          </hw-list-item-meta>
        </hw-list-item>
      </hw-list>

    </hw-card>
  </div>
</template>
<script lang="ts">
import {defineComponent} from 'vue';


  import {
    SearchIcon
  } from '@/ui/icons';
  import {
    wafStatRuntimeSysinfoapi
  } from '@/apis/stats';


  export default defineComponent({
    name: 'ListBase',
    components: {
    },
    data() {
      return {
        runtimes: [],
      };
    },
    computed: {
    },
    mounted() {
      this.loadRunTimeSysinfo()
    },

    methods: {
      loadRunTimeSysinfo(){
        wafStatRuntimeSysinfoapi("").then((res)=>{
          console.log(res)
          this.runtimes = res.data
        }).catch().finally()
      },
    },
  });
</script>

<style lang="less" scoped>
  @import '@/style/variables';

  .payment-col {
    display: flex;

    .trend-container {
      display: flex;
      align-items: center;
      margin-left: 8px;
    }
  }

  .left-operation-container {
    padding: 0 0 6px 0;
    margin-bottom: 16px;

    .selected-count {
      display: inline-block;
      margin-left: 8px;
      color: var(--td-text-color-secondary);
    }
  }

  .search-input {
    width: 360px;
  }

  .t-button+.t-button {
    margin-left: @spacer;
  }
</style>
