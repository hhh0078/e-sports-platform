<!-- 文件作用：平台文章管理入口，公开、草稿和下架文章按状态分页。 -->
<template><AppShell title="文章管理" back><ArticleListView admin/></AppShell></template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import ArticleListView from '../../../components/ArticleListView.vue'
</script>
