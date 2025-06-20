<template>
  <div class="flex flex-col space-y-4">
    <div class="flex items-center justify-between px-4">
      <div class="flex items-center space-x-2">
        <SearchBox
          v-model:value="state.searchText"
          style="max-width: 100%"
          :placeholder="$t('common.filter-by-name')"
        />
      </div>
      <NButton
        v-if="hasWorkspacePermissionV2('bb.projects.create')"
        type="primary"
        @click="state.showCreateDrawer = true"
      >
        <template #icon>
          <PlusIcon class="h-4 w-4" />
        </template>
        {{ $t("quick-action.new-project") }}
      </NButton>
    </div>
    <div class="space-y-2">
      <ProjectOperations
        v-if="hasWorkspacePermissionV2('bb.projects.delete')"
        :project-list="selectedProjectList"
        @update="handleBatchOperation"
      />
      <PagedProjectTable
        ref="pagedProjectTableRef"
        session-key="bb.project-table"
        :filter="filter"
        :bordered="false"
        :footer-class="'mx-4'"
        :prevent-default="!!onRowClick"
        :selected-project-names="selectedProjectNames"
        @update:selected-project-names="updateSelectedProjects"
        @row-click="onRowClick"
      />
    </div>
  </div>
  <Drawer
    :auto-focus="true"
    :close-on-esc="true"
    :show="state.showCreateDrawer"
    @close="state.showCreateDrawer = false"
  >
    <ProjectCreatePanel
      :on-created="handleCreated"
      @dismiss="state.showCreateDrawer = false"
    />
  </Drawer>
</template>

<script lang="ts" setup>
import { PlusIcon } from "lucide-vue-next";
import { NButton } from "naive-ui";
import { computed, onMounted, reactive, ref } from "vue";
import type { ComponentExposed } from "vue-component-type-helpers";
import { useRouter } from "vue-router";
import ProjectCreatePanel from "@/components/Project/ProjectCreatePanel.vue";
import { SearchBox, PagedProjectTable } from "@/components/v2";
import ProjectOperations from "@/components/v2/Model/Project/ProjectOperations.vue";
import { Drawer } from "@/components/v2";
import { useProjectV1Store, useUIStateStore } from "@/store";
import type { ComposedProject } from "@/types";
import { hasWorkspacePermissionV2 } from "@/utils";

interface LocalState {
  searchText: string;
  showCreateDrawer: boolean;
  selectedProjects: Set<string>;
}

const props = defineProps<{
  onRowClick?: (project: ComposedProject) => void;
}>();

const state = reactive<LocalState>({
  searchText: "",
  showCreateDrawer: false,
  selectedProjects: new Set(),
});

const router = useRouter();
const projectStore = useProjectV1Store();

const pagedProjectTableRef = ref<ComponentExposed<typeof PagedProjectTable>>();

const filter = computed(() => ({
  query: state.searchText,
  excludeDefault: true,
}));

const selectedProjectNames = computed(() => {
  return Array.from(state.selectedProjects);
});

const selectedProjectList = computed(() => {
  if (state.selectedProjects.size === 0) {
    return [];
  }
  return Array.from(state.selectedProjects)
    .map((name) => projectStore.getProjectByName(name))
    .filter((p): p is ComposedProject => p !== undefined);
});

const updateSelectedProjects = (projectNames: string[]) => {
  state.selectedProjects = new Set(projectNames);
};

const handleBatchOperation = () => {
  state.selectedProjects.clear();
  pagedProjectTableRef.value?.refresh();
};

onMounted(() => {
  const uiStateStore = useUIStateStore();
  if (!uiStateStore.getIntroStateByKey("project.visit")) {
    uiStateStore.saveIntroStateByKey({
      key: "project.visit",
      newState: true,
    });
  }
});

const handleCreated = async (project: ComposedProject) => {
  if (props.onRowClick) {
    return props.onRowClick(project);
  }
  const url = {
    path: `/${project.name}`,
  };
  router.push(url);
  state.showCreateDrawer = false;
};
</script>
