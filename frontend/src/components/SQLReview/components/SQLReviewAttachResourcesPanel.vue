<template>
  <Drawer :show="show" @close="$emit('close')">
    <DrawerContent
      :title="$t('sql-review.attach-resource.self')"
      class="w-[40rem] max-w-[100vw] relative"
    >
      <template #default>
        <div class="space-y-6">
          <p class="textinfolabel">
            {{ $t("sql-review.attach-resource.label") }}
          </p>
          <div>
            <div class="textlabel mb-1">
              {{ $t("common.environment") }}
            </div>
            <p class="textinfolabel">
              {{ $t("sql-review.attach-resource.label-environment") }}
            </p>
            <EnvironmentSelect
              class="mt-3"
              required
              :environment-names="environmentNames"
              :multiple="true"
              :render-suffix="getResourceAttachedConfigName"
              @update:environment-names="
                onResourcesChange($event, projectNames)
              "
            />
          </div>
          <div class="flex items-center space-x-2">
            <div class="textlabel w-10 capitalize">{{ $t("common.or") }}</div>
            <NDivider />
          </div>
          <div>
            <div class="textlabel mb-1">
              {{ $t("common.project") }}
            </div>
            <p class="textinfolabel">
              {{ $t("sql-review.attach-resource.label-project") }}
            </p>
            <ProjectSelect
              class="mt-3"
              style="width: 100%"
              required
              :project-names="projectNames"
              :multiple="true"
              :render-suffix="getResourceAttachedConfigName"
              @update:project-names="
                onResourcesChange($event, environmentNames)
              "
            />
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex items-center justify-end gap-x-3">
          <NButton @click="$emit('close')">
            {{ $t("common.cancel") }}
          </NButton>
          <NButton :disabled="!hasPermission" type="primary" @click="onConfirm">
            {{ $t("common.confirm") }}
          </NButton>
        </div>
      </template>
    </DrawerContent>
  </Drawer>

  <ResourceOccupiedModal
    ref="resourceOccupiedModalRef"
    :target="review.name"
    :description="
      $t('sql-review.attach-resource.override-warning', {
        button: t('common.continue-anyway'),
      })
    "
    :resources="resourcesOccupied"
    :show-positive-button="true"
    @on-submit="onSubmit"
  />
</template>

<script setup lang="tsx">
import { cloneDeep } from "lodash-es";
import { NButton, NDivider } from "naive-ui";
import { watch, ref, computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  Drawer,
  DrawerContent,
  EnvironmentSelect,
  ProjectSelect,
} from "@/components/v2";
import ResourceOccupiedModal from "@/components/v2/ResourceOccupiedModal/ResourceOccupiedModal.vue";
import { useSQLReviewStore, pushNotification } from "@/store";
import {
  environmentNamePrefix,
  projectNamePrefix,
} from "@/store/modules/v1/common";
import type { SQLReviewPolicy } from "@/types";
import { hasWorkspacePermissionV2 } from "@/utils";

const props = defineProps<{
  show: boolean;
  review: SQLReviewPolicy;
}>();

const emit = defineEmits<{
  (event: "close"): void;
}>();

const resources = ref<string[]>([]);
const sqlReviewStore = useSQLReviewStore();
const { t } = useI18n();
const resourceOccupiedModalRef =
  ref<InstanceType<typeof ResourceOccupiedModal>>();

watch(
  () => props.review.resources,
  () => {
    resources.value = [...props.review.resources];
  },
  { immediate: true, deep: true }
);

const hasPermission = computed(() => {
  return hasWorkspacePermissionV2("bb.policies.update");
});

const environmentNames = computed(() =>
  resources.value.filter((resource) =>
    resource.startsWith(environmentNamePrefix)
  )
);

const projectNames = computed(() =>
  resources.value.filter((resource) => resource.startsWith(projectNamePrefix))
);

const onResourcesChange = (
  newResources: string[],
  otherResources: string[]
) => {
  resources.value = [...newResources, ...otherResources];
};

const resourcesBindingWithOtherPolicy = computed(() => {
  const map = new Map<string, string[]>();
  for (const resource of resources.value) {
    const config = sqlReviewStore.getReviewPolicyByResouce(resource);
    if (config && config.id !== props.review.id) {
      if (!map.has(config.id)) {
        map.set(config.id, []);
      }
      map.get(config.id)?.push(resource);
    }
  }
  return map;
});

const resourcesOccupied = computed(() =>
  [...resourcesBindingWithOtherPolicy.value.values()].reduce(
    (list, resources) => {
      list.push(...resources);
      return list;
    },
    []
  )
);

const onConfirm = async () => {
  if (resourcesOccupied.value.length === 0) {
    await upsertReviewResource();
  } else {
    resourceOccupiedModalRef.value?.open();
  }
};

const onSubmit = () => {
  const map = cloneDeep(resourcesBindingWithOtherPolicy.value);
  upsertReviewResource().then(() => {
    sqlReviewStore.removeResourceForReview(map);
  });
};

const upsertReviewResource = async () => {
  await sqlReviewStore.upsertReviewConfigTag({
    oldResources: props.review.resources,
    newResources: resources.value,
    review: props.review.id,
  });
  pushNotification({
    module: "bytebase",
    style: "SUCCESS",
    title: t("sql-review.policy-updated"),
  });
  emit("close");
};

const getResourceAttachedConfigName = (resource: string) => {
  const config = sqlReviewStore.getReviewPolicyByResouce(resource)?.name;
  return config ? `(${config})` : "";
};
</script>
