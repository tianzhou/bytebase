<template>
  <div ref="containerElRef" class="w-full h-full px-2 py-2 overflow-x-auto">
    <NDataTable
      v-bind="$attrs"
      ref="dataTableRef"
      size="small"
      :row-key="
        ({ package: pack, position }) => keyWithPosition(pack.name, position)
      "
      :columns="columns"
      :data="layoutReady ? filteredPackages : []"
      :row-props="rowProps"
      :max-height="tableBodyHeight"
      :virtual-scroll="true"
      :striped="true"
      :bordered="true"
      :bottom-bordered="true"
      row-class-name="cursor-pointer"
    />
  </div>
</template>

<script setup lang="tsx">
import { NDataTable, type DataTableColumn } from "naive-ui";
import { computed, h, watch } from "vue";
import { useI18n } from "vue-i18n";
import type { ComposedDatabase } from "@/types";
import type {
  DatabaseMetadata,
  PackageMetadata,
  SchemaMetadata,
} from "@/types/proto-es/v1/database_service_pb";
import { getHighlightHTMLByRegExp, useAutoHeightDataTable } from "@/utils";
import { keyWithPosition } from "@/views/sql-editor/EditorCommon";
import { useCurrentTabViewStateContext } from "../../context/viewState";

type PackageWithPosition = {
  package: PackageMetadata;
  position: number;
};

const props = defineProps<{
  db: ComposedDatabase;
  database: DatabaseMetadata;
  schema: SchemaMetadata;
  packages: PackageMetadata[];
  keyword?: string;
  maxHeight?: number;
}>();

const emit = defineEmits<{
  (
    event: "click",
    metadata: {
      database: DatabaseMetadata;
      schema: SchemaMetadata;
      package: PackageMetadata;
      position: number;
    }
  ): void;
}>();

const { t } = useI18n();
const { viewState } = useCurrentTabViewStateContext();

const packagesWithPosition = computed(() => {
  return props.packages.map<PackageWithPosition>((pack, position) => ({
    package: pack,
    position,
  }));
});

const filteredPackages = computed(() => {
  const keyword = props.keyword?.trim().toLowerCase();
  if (keyword) {
    return packagesWithPosition.value.filter(({ package: pack }) =>
      pack.name.toLowerCase().includes(keyword)
    );
  }
  return packagesWithPosition.value;
});

const {
  dataTableRef,
  containerElRef,
  virtualListRef,
  tableBodyHeight,
  layoutReady,
} = useAutoHeightDataTable(
  filteredPackages,
  computed(() => ({
    maxHeight: props.maxHeight ? props.maxHeight : null,
  }))
);

const columns = computed(() => {
  const columns: (DataTableColumn<PackageWithPosition> & {
    hide?: boolean;
  })[] = [
    {
      key: "name",
      title: t("schema-editor.database.name"),
      resizable: true,
      className: "truncate",
      render: ({ package: pack }) => {
        return h("span", {
          innerHTML: getHighlightHTMLByRegExp(pack.name, props.keyword ?? ""),
        });
      },
    },
  ];
  return columns;
});

const rowProps = ({ package: pack, position }: PackageWithPosition) => {
  return {
    onClick: () => {
      emit("click", {
        database: props.database,
        schema: props.schema,
        package: pack,
        position,
      });
    },
  };
};

watch(
  [() => viewState.value?.detail.package, virtualListRef],
  ([pack, vl]) => {
    if (pack && vl) {
      vl.scrollTo({ key: pack });
    }
  },
  { immediate: true }
);
</script>

<style lang="postcss" scoped>
:deep(.n-data-table-th .n-data-table-resize-button::after) {
  @apply bg-control-bg h-2/3;
}
:deep(.n-data-table-td.input-cell) {
  @apply pl-0.5 pr-1 py-0;
}
</style>
