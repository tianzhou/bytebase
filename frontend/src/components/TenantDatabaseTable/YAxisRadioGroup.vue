<template>
  <NSelect
    v-if="visible"
    :options="options"
    :value="label"
    style="width: 9rem"
    v-bind="$attrs"
    @update-value="$emit('update:label', $event)"
  />
</template>

<script lang="ts" setup>
import type { SelectOption } from "naive-ui";
import { NSelect } from "naive-ui";
import { computed } from "vue";
import type { ComposedDatabase } from "@/types";
import {
  displayDeploymentMatchSelectorKey,
  getAvailableDeploymentConfigMatchSelectorKeyList,
} from "@/utils";

const props = withDefaults(
  defineProps<{
    databaseList: ComposedDatabase[];
    label: string;
    excludedKeyList?: string[];
  }>(),
  {
    excludedKeyList: () => [],
  }
);

defineEmits<{
  (event: "update:label", label: string): void;
}>();

const labelKeyList = computed(() => {
  return getAvailableDeploymentConfigMatchSelectorKeyList(
    props.databaseList,
    true /* withVirtualLabelKeys */,
    true /* sort */
  ).filter((key) => !props.excludedKeyList.includes(key));
});

const visible = computed(() => {
  if (!props.label) return false;
  return labelKeyList.value.includes(props.label);
});
const options = computed(() => {
  return labelKeyList.value.map<SelectOption>((key) => {
    return {
      value: key,
      label: displayDeploymentMatchSelectorKey(key),
    };
  });
});
</script>
