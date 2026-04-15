<script setup>
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { DeletePassword, UpdatePassword, ToggleFavorite } from '../../wailsjs/go/main/App'

const props = defineProps({
  passwords: { type: Array, default: () => [] },
  showPwdMap: { type: Object, default: () => ({}) },
  sortBy: { type: String, default: 'created' }
})

const emit = defineEmits([
  'refresh',
  'edit',
  'toggle-pwd',
  'copy-pwd',
  'sort-change'
])

const viewMode = ref('list') // 'grid' | 'list'

// 获取网站图标
function getFavicon(url) {
  if (!url) return ''
  try {
    const u = new URL(url)
    return `https://www.google.com/s2/favicons?domain=${u.hostname}&sz=64`
  } catch {
    return ''
  }
}

// 打开编辑
function handleEdit(row) {
  emit('edit', row)
}

// 删除
async function handleDelete(row) {
  try {
    await ElMessageBox.confirm('确定要删除该密码吗？', '提示', { type: 'warning' })
    await DeletePassword(row.id)
    ElMessage.success('删除成功！')
    emit('refresh')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || String(e))
    }
  }
}

// 收藏
async function handleToggleFavorite(row) {
  try {
    await ToggleFavorite(row.id)
    emit('refresh')
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

// 切换显示密码
function toggleShowPwd(id) {
  emit('toggle-pwd', id)
}

// 复制密码
function copyPassword(pwd) {
  emit('copy-pwd', pwd)
}
</script>

<template>
  <div class="password-list">
    <!-- 工具栏 -->
    <div class="toolbar">
      <el-radio-group v-model="viewMode" size="small">
        <el-radio-button label="list">
          <el-icon><List /></el-icon> 列表
        </el-radio-button>
        <el-radio-button label="grid">
          <el-icon><Grid /></el-icon> 网格
        </el-radio-button>
      </el-radio-group>

      <el-radio-group :model-value="props.sortBy" size="small" @change="(val) => emit('sort-change', val)">
        <el-radio-button label="title">名称</el-radio-button>
        <el-radio-button label="created">最近添加</el-radio-button>
        <el-radio-button label="updated">最近修改</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 网格视图 -->
    <div v-if="viewMode === 'grid'" class="grid-view">
      <el-card
        v-for="item in passwords"
        :key="item.id"
        class="password-card"
        :class="{ favorite: item.isFavorite }"
        shadow="hover"
      >
        <div class="card-header">
          <img v-if="getFavicon(item.url)" :src="getFavicon(item.url)" class="site-icon" />
          <div v-else class="site-icon placeholder">{{ item.title ? item.title[0] : '?' }}</div>
          <el-button
            circle
            size="small"
            :type="item.isFavorite ? 'warning' : 'default'"
            @click="handleToggleFavorite(item)"
          >
            <el-icon><Star /></el-icon>
          </el-button>
        </div>

        <h4 class="title" :title="item.title">{{ item.title }}</h4>
        <p class="meta" :title="item.username">{{ item.username }}</p>
        <p v-if="item.url" class="url" :title="item.url">{{ item.url }}</p>

        <div class="password-row">
          <span v-if="!showPwdMap[item.id]" class="pwd-mask">********</span>
          <span v-else class="pwd-text">{{ item.password }}</span>
          <el-button size="small" text @click="toggleShowPwd(item.id)">
            {{ showPwdMap[item.id] ? '隐藏' : '显示' }}
          </el-button>
        </div>

        <div class="actions">
          <el-button size="small" type="primary" @click="copyPassword(item.password)">复制</el-button>
          <el-button size="small" @click="handleEdit(item)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(item)">删除</el-button>
        </div>
      </el-card>
    </div>

    <!-- 列表视图 -->
    <el-table v-else :data="passwords" style="width: 100%;" stripe>
      <el-table-column width="60" align="center">
        <template #default="{ row }">
          <img v-if="getFavicon(row.url)" :src="getFavicon(row.url)" class="list-icon" />
          <div v-else class="list-icon placeholder">{{ row.title ? row.title[0] : '?' }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="title" label="标题" min-width="120" />
      <el-table-column prop="url" label="网址" min-width="140">
        <template #default="{ row }">
          <a v-if="row.url" :href="row.url" target="_blank" class="link">{{ row.url }}</a>
          <span v-else class="empty">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="username" label="用户名" min-width="120" />
      <el-table-column prop="password" label="密码" min-width="200">
        <template #default="{ row }">
          <span v-if="!showPwdMap[row.id]">********</span>
          <span v-else>{{ row.password }}</span>
          <el-button size="small" text @click="toggleShowPwd(row.id)">
            {{ showPwdMap[row.id] ? '隐藏' : '显示' }}
          </el-button>
          <el-button size="small" type="primary" text @click="copyPassword(row.password)">复制</el-button>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="handleEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          <el-button
            size="small"
            :type="row.isFavorite ? 'warning' : 'default'"
            @click="handleToggleFavorite(row)"
          >
            <el-icon><Star /></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="passwords.length === 0" class="empty-state">
      <el-empty description="暂无密码条目" />
    </div>
  </div>
</template>

<style scoped>
.password-list {
  width: 100%;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.grid-view {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}
.password-card {
  transition: all 0.2s;
}
.password-card.favorite {
  border: 1px solid #f0c040;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.site-icon {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  object-fit: contain;
}
.site-icon.placeholder {
  background: #e4e7ed;
  color: #606266;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  font-size: 16px;
}
.title {
  margin: 0 0 6px 0;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.meta {
  margin: 0 0 4px 0;
  font-size: 13px;
  color: #606266;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.url {
  margin: 0 0 10px 0;
  font-size: 12px;
  color: #409eff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.password-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.pwd-mask {
  font-family: monospace;
  letter-spacing: 1px;
}
.pwd-text {
  font-family: monospace;
  font-size: 13px;
  word-break: break-all;
  flex: 1;
}
.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.list-icon {
  width: 24px;
  height: 24px;
  border-radius: 4px;
  object-fit: contain;
}
.list-icon.placeholder {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #e4e7ed;
  color: #606266;
  font-size: 12px;
  font-weight: bold;
}
.link {
  color: #409eff;
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: inline-block;
  max-width: 100%;
}
.link:hover {
  text-decoration: underline;
}
.empty {
  color: #909399;
}
.empty-state {
  padding: 40px 0;
}
</style>
