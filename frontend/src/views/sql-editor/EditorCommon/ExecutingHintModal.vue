<template>
  <BBModal
    v-if="sqlEditorStore.isShowExecutingHint"
    :title="$t('common.tips')"
    @close="handleClose"
  >
    <ExecuteHint
      :database="sqlEditorStore.executingHintDatabase"
      @close="handleClose"
    />
  </BBModal>
</template>

<script lang="ts" setup>
import { BBModal } from "@/bbkit";
import { useSQLEditorStore } from "@/store";
import ExecuteHint from "./ExecuteHint.vue";

const sqlEditorStore = useSQLEditorStore();

const handleClose = () => {
  sqlEditorStore.isShowExecutingHint = false;
};
</script>
