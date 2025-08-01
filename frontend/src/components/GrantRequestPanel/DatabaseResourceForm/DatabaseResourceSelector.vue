<template>
  <div class="w-full relative space-y-2">
    <AdvancedSearch
      v-model:params="params"
      :autofocus="false"
      :placeholder="$t('database.filter-database')"
      :scope-options="scopeOptions"
      :override-route-query="false"
    />
    <NTransfer
      id="database-resource-selector"
      v-model:value="selectedValueList"
      style="height: 512px"
      :disabled="disabled"
      :options="sourceTransferOptions"
      :render-source-list="renderSourceList"
      :render-target-list="renderTargetList"
    />
    <div
      v-if="initializing"
      class="z-1 absolute inset-0 bg-white bg-opacity-80 flex flex-row justify-center items-center"
    >
      <BBSpin size="large" />
    </div>
  </div>
</template>

<script setup lang="tsx">
import { useDebounceFn } from "@vueuse/core";
import { orderBy, uniqBy } from "lodash-es";
import type { TransferRenderSourceList, TreeOption } from "naive-ui";
import { NTransfer, NTree, NButton } from "naive-ui";
import { computed, h, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { BBSpin } from "@/bbkit";
import AdvancedSearch from "@/components/AdvancedSearch";
import { useCommonSearchScopeOptions } from "@/components/AdvancedSearch/useCommonSearchScopeOptions";
import {
  type DatabaseFilter,
  useDatabaseV1Store,
  useDBSchemaV1Store,
  batchGetOrFetchDatabases,
} from "@/store";
import {
  instanceNamePrefix,
  environmentNamePrefix,
} from "@/store/modules/v1/common";
import {
  DEBOUNCE_SEARCH_DELAY,
  type ComposedDatabase,
  type DatabaseResource,
} from "@/types";
import {
  getDefaultPagination,
  type SearchParams,
  CommonFilterScopeIdList,
  extractProjectResourceName,
} from "@/utils";
import { convertScopeValueToEngine } from "@/utils/v1/common-conversions";
import Label from "./Label.vue";
import type { DatabaseTreeOption } from "./common";
import {
  flattenTreeOptions,
  getSchemaOrTableTreeOptions,
  mapTreeOptions,
  parseStringToResource,
} from "./common";

const props = defineProps<{
  disabled?: boolean;
  projectName: string;
  includeCloumn: boolean;
  databaseResources: DatabaseResource[];
}>();

const emit = defineEmits<{
  (
    e: "update:databaseResources",
    databaseResourceList: DatabaseResource[]
  ): void;
}>();

const databaseStore = useDatabaseV1Store();
const dbSchemaStore = useDBSchemaV1Store();
const { t } = useI18n();

const scopeOptions = useCommonSearchScopeOptions([
  ...CommonFilterScopeIdList,
  "project",
  "database-label",
  "engine",
  "table",
]);

const parseResourceToKey = (resource: DatabaseResource): string => {
  const data = [
    resource.databaseFullName,
    "schemas",
    resource.schema,
    "tables",
    resource.table,
    "columns",
    resource.column,
  ];

  while (data.length > 0) {
    const item = data.pop();
    if (!item) {
      data.pop();
      continue;
    }
    data.push(item);
    break;
  }

  return data.join("/");
};

const getInitParams = (): SearchParams => {
  return {
    query: "",
    scopes: [
      {
        id: "project",
        value: extractProjectResourceName(props.projectName),
        readonly: true,
      },
    ],
  };
};

const params = ref<SearchParams>(getInitParams());
watch(
  () => props.projectName,
  () => (params.value = getInitParams()),
  { immediate: true }
);

const selectedValueList = ref<string[]>([]);
const expandedKeys = ref<string[]>([]);
const indeterminateKeys = ref<string[]>([]);
const initializing = ref(true);
const databaseList = ref<ComposedDatabase[]>([]);
const fetchDataState = ref<{
  nextPageToken?: string;
  loading: boolean;
}>({
  loading: false,
});

const cascadeLoopTreeNode = (
  treeNode: DatabaseTreeOption,
  callback: (node: DatabaseTreeOption) => void
) => {
  callback(treeNode);
  for (const child of treeNode?.children ?? []) {
    cascadeLoopTreeNode(child, callback);
  }
};

const selectedInstance = computed(() => {
  const instanceId = params.value.scopes.find(
    (scope) => scope.id === "instance"
  )?.value;
  if (!instanceId) {
    return;
  }
  return `${instanceNamePrefix}${instanceId}`;
});

const selectedEnvironment = computed(() => {
  const environmentId = params.value.scopes.find(
    (scope) => scope.id === "environment"
  )?.value;
  if (!environmentId) {
    return;
  }
  return `${environmentNamePrefix}${environmentId}`;
});

const selectedTable = computed(() => {
  return params.value.scopes.find((scope) => scope.id === "table")?.value;
});

const collectExpandedKeys = async ({
  database,
  table,
}: {
  database: string;
  table: string;
}) => {
  const databaseMetadata = await dbSchemaStore.getOrFetchDatabaseMetadata({
    database,
  });
  if (!databaseMetadata) {
    return;
  }
  expandedKeys.value.push(database);
  for (const schema of databaseMetadata.schemas) {
    expandedKeys.value.push(`${database}/schemas/${schema.name}`);
    for (const t of schema.tables) {
      if (t.name === table) {
        expandedKeys.value.push(
          `${database}/schemas/${schema.name}/tables/${t.name}`
        );
      }
    }
  }
};

const filterTableList = computed(() => {
  if (!selectedTable.value) {
    return undefined;
  }

  return expandedKeys.value;
});

const selectedLabels = computed(() => {
  return params.value.scopes
    .filter((scope) => scope.id === "database-label")
    .map((scope) => scope.value);
});

const selectedEngines = computed(() => {
  return params.value.scopes
    .filter((scope) => scope.id === "engine")
    .map((scope) => convertScopeValueToEngine(scope.value));
});

const databaseFilter = computed(
  (): DatabaseFilter => ({
    instance: selectedInstance.value,
    environment: selectedEnvironment.value,
    query: params.value.query,
    labels: selectedLabels.value,
    engines: selectedEngines.value,
    table: selectedTable.value,
  })
);

const fetchDatabaseList = useDebounceFn(async () => {
  fetchDataState.value.loading = true;
  const pageToken = fetchDataState.value.nextPageToken;

  try {
    const { databases, nextPageToken } = await databaseStore.fetchDatabases({
      pageSize: getDefaultPagination(),
      pageToken,
      parent: props.projectName,
      filter: databaseFilter.value,
    });

    if (pageToken) {
      databaseList.value.push(...databases);
      databaseList.value = uniqBy(databaseList.value, (db) => db.name);
    } else {
      databaseList.value = databases;
    }
    fetchDataState.value.nextPageToken = nextPageToken;
  } finally {
    fetchDataState.value.loading = false;
  }
}, DEBOUNCE_SEARCH_DELAY);

watch(
  () => databaseFilter.value,
  async () => {
    fetchDataState.value.nextPageToken = "";
    expandedKeys.value = [];
    await fetchDatabaseList();
    if (!params.value.query && params.value.scopes.length === 1) {
      databaseList.value = uniqBy(
        [
          ...databaseList.value,
          ...props.databaseResources.map((resource) =>
            databaseStore.getDatabaseByName(resource.databaseFullName)
          ),
        ],
        (db) => db.name
      );
    }

    if (databaseFilter.value.table) {
      // expand all
      for (const database of databaseList.value) {
        await collectExpandedKeys({
          database: database.name,
          table: databaseFilter.value.table,
        });
      }
    }
  },
  {
    deep: true,
    immediate: true,
  }
);

onMounted(async () => {
  await batchGetOrFetchDatabases(
    props.databaseResources.map((resource) => resource.databaseFullName)
  );

  const selectedKeys = props.databaseResources.map(parseResourceToKey);
  const databaseNames = new Set(
    selectedKeys.map((key) => key.split("/schemas/")[0]).filter((key) => key)
  );
  await Promise.all(
    [...databaseNames].map(async (databaseName) => {
      await dbSchemaStore.getOrFetchDatabaseMetadata({
        database: databaseName,
      });
    })
  );

  const newCheckedKeys = new Set(selectedKeys);
  const newIndeterminateKeys = new Set<string>([]);
  const newExpandedKeys = new Set(
    // expand parents for selected keys
    selectedKeys
      .map((key) => {
        const pathes = parseKeyToPathes(key);
        // key: {db}/schemas/{schema}
        // expaned: [{db}]
        //
        // key: {db}/schemas/{schema}/tables/{table}
        // expaned: [{db}, {db}/schemas/{schema}]
        //
        // key: {db}/schemas/{schema}/tables/{table}/columns/{column}
        // expaned: [{db}, {db}/schemas/{schema}, {db}/schemas/{schema}/tables/{table}]
        pathes.pop();
        return pathes;
      })
      .flat()
  );

  for (const selectedKey of selectedKeys) {
    const checkedNode = sourceTransferOptions.value.find(
      (option) => option.value === selectedKey
    );
    if (!checkedNode) {
      continue;
    }
    // loop to check and expand all children
    cascadeLoopTreeNode(checkedNode, (treeNode) => {
      newCheckedKeys.add(treeNode.value);
      if (treeNode.children) {
        newExpandedKeys.add(treeNode.value);
      }
    });

    // add parent pathes to indeterminate keys
    const parentPathes = parseKeyToPathes(checkedNode.value);
    parentPathes.pop();
    while (parentPathes.length > 0) {
      const parentPath = parentPathes.pop() as string;
      // move the parent to the indeterminate keys.
      if (!newCheckedKeys.has(parentPath)) {
        newIndeterminateKeys.add(parentPath);
      }
    }
  }

  selectedValueList.value = [...newCheckedKeys];
  expandedKeys.value = [...newExpandedKeys];
  indeterminateKeys.value = [...newIndeterminateKeys];

  initializing.value = false;
});

const parseKeyToPathes = (key: string): string[] => {
  if (!key) {
    return [];
  }

  const sections = key.split("/");
  const nodePrefx = new Set(["schemas", "tables", "columns"]);
  const resp: string[] = [];
  const tmp: string[] = [];

  for (const section of sections) {
    if (nodePrefx.has(section)) {
      resp.push(tmp.join("/"));
    }
    tmp.push(section);
  }

  if (tmp.length > 0) {
    resp.push(tmp.join("/"));
  }

  return resp;
};

const sourceTreeOptions = computed(() => {
  return mapTreeOptions({
    databaseList: databaseList.value,
    includeCloumn: props.includeCloumn,
    filterValueList: filterTableList.value,
  });
});

const sourceTransferOptions = computed((): DatabaseTreeOption[] => {
  const options = flattenTreeOptions(sourceTreeOptions.value);
  return options;
});

const onTreeNodeLoad = async (node: TreeOption) => {
  const treeNode = node as DatabaseTreeOption;
  if (treeNode.level === "databases") {
    await dbSchemaStore.getOrFetchDatabaseMetadata({
      database: treeNode.value,
    });
    const database = await databaseStore.getOrFetchDatabaseByName(
      treeNode.value
    );
    const children = getSchemaOrTableTreeOptions({
      database,
      includeCloumn: props.includeCloumn,
    });
    if (children && children.length > 0) {
      treeNode.children = children;
      treeNode.isLeaf = false;
    } else {
      treeNode.isLeaf = true;
    }
  }
};

const renderSourceList: TransferRenderSourceList = ({ onCheck, pattern }) => {
  return h(
    "div",
    { class: "flex flex-col space-y-2 pb-4" },
    {
      default: () => [
        h(NTree, {
          keyField: "value",
          cascade: true,
          allowCheckingNotLoaded: true,
          checkable: true,
          selectable: false,
          checkOnClick: true,
          disabled: props.disabled,
          data: sourceTreeOptions.value,
          blockLine: true,
          renderLabel: ({ option }: { option: TreeOption }) => {
            return h(Label, {
              option: option as DatabaseTreeOption,
              keyword: pattern,
            });
          },
          pattern,
          showIrrelevantNodes: false,
          expandedKeys: expandedKeys.value,
          checkedKeys: selectedValueList.value,
          indeterminateKeys: indeterminateKeys.value,
          onLoad: onTreeNodeLoad,
          onUpdateExpandedKeys: (keys: string[]) => {
            expandedKeys.value = keys;
          },
          onUpdateCheckedKeys: async (
            checkedKeys: string[],
            _: Array<TreeOption | null>,
            meta: { node: TreeOption | null; action: "check" | "uncheck" }
          ) => {
            if (!meta.node) {
              return;
            }

            const oldIndeterminateKeys = new Set(indeterminateKeys.value);
            const newCheckedKeys = new Set(checkedKeys);
            const oldCheckedKeys = new Set(selectedValueList.value);
            const treeNode = meta.node as DatabaseTreeOption;

            const checkNodeAndAllChildren = async () => {
              await onTreeNodeLoad(treeNode);
              // refresh node in case the schema is updated
              const checkedNode = sourceTransferOptions.value.find(
                (option) => option.value === treeNode.value
              );
              if (checkedNode) {
                // check and expand all children
                cascadeLoopTreeNode(checkedNode, (treeNode) => {
                  newCheckedKeys.add(treeNode.value);
                  if (treeNode.children) {
                    expandedKeys.value.push(treeNode.value);
                  }
                });
              }
            };

            if (meta.action === "check") {
              oldIndeterminateKeys.delete(treeNode.value);
              await checkNodeAndAllChildren();

              const parentPathes = parseKeyToPathes(treeNode.value);
              parentPathes.pop();
              while (parentPathes.length > 0) {
                const parentPath = parentPathes.pop() as string;
                // If users not manually select the parent,
                // then DONOT check the parent,
                // move the parent to the indeterminate keys instead.
                if (
                  !oldCheckedKeys.has(parentPath) &&
                  newCheckedKeys.has(parentPath)
                ) {
                  newCheckedKeys.delete(parentPath);
                  oldIndeterminateKeys.add(parentPath);
                }
              }
            } else {
              if (oldIndeterminateKeys.has(treeNode.value)) {
                // uncheck an indeterminate key should be check
                oldIndeterminateKeys.delete(treeNode.value);

                await checkNodeAndAllChildren();
              } else {
                // loop parent pathes to check if we need to update the indeterminate keys
                const parentPathes = parseKeyToPathes(treeNode.value);
                parentPathes.pop();
                while (parentPathes.length > 0) {
                  const parentPath = parentPathes.pop() as string;
                  if (!oldIndeterminateKeys.has(parentPath)) {
                    continue;
                  }
                  if (
                    !checkedKeys.find((key) => key.startsWith(`${parentPath}/`))
                  ) {
                    oldIndeterminateKeys.delete(parentPath);
                  }
                }
              }
            }

            selectedValueList.value = [...newCheckedKeys];
            onCheck([...newCheckedKeys]);
            indeterminateKeys.value = [...oldIndeterminateKeys];
          },
        }),
        fetchDataState.value.nextPageToken
          ? h(
              "div",
              { class: "w-full flex items-center justify-center" },
              h(
                NButton,
                {
                  quaternary: true,
                  size: "small",
                  loading: fetchDataState.value.loading,
                  onClick: fetchDatabaseList,
                },
                {
                  default: () => t("common.load-more"),
                }
              )
            )
          : undefined,
      ],
    }
  );
};

const targetTreeOptions = computed(() => {
  if (selectedValueList.value.length === 0) {
    return [];
  }

  const nodes = mapTreeOptions({
    databaseList: databaseList.value,
    filterValueList: selectedValueList.value,
    includeCloumn: props.includeCloumn,
  });

  for (const databaseNode of nodes) {
    if (!databaseNode.children || databaseNode.children.length === 0) {
      databaseNode.isLeaf = true;
    }
  }
  return nodes;
});

const renderTargetList: TransferRenderSourceList = () => {
  return h(NTree, {
    keyField: "value",
    checkable: false,
    selectable: false,
    defaultExpandAll: true,
    disabled: props.disabled,
    data: targetTreeOptions.value,
    blockLine: true,
    virtualScroll: true,
    style: "height: 468px", // since <NTransfer> height is 512
    renderLabel: ({ option }: { option: TreeOption }) => {
      const node = option as DatabaseTreeOption;
      return (
        <Label
          option={node}
          class={
            selectedValueList.value.includes(node.value)
              ? "text-indigo-700 font-medium"
              : "textinfolabel"
          }
        />
      );
    },
    showIrrelevantNodes: false,
    checkedKeys: selectedValueList.value,
  });
};

watch(selectedValueList, (selectedValueList) => {
  const orderedList = orderBy(selectedValueList, (item) => item.length, "asc");
  const filteredKeyList: string[] = [];
  for (const key of orderedList) {
    const parentExisted = filteredKeyList.some((parent) =>
      key.startsWith(`${parent}/`)
    );
    // If the parent node is selected, means all children should be selected.
    // So we can ignore the children.
    // For example, select table "employee"."public"."employee" and all its fields "emp_no" & "name",
    // we only need the "employee"."public"."employee" to build the database resource,
    // and the expression only need table level too (ignore the column means column = "*")
    if (!parentExisted) {
      filteredKeyList.push(key);
    }
  }

  emit(
    "update:databaseResources",
    filteredKeyList
      .map(parseStringToResource)
      .filter((data) => data) as DatabaseResource[]
  );
});
</script>
