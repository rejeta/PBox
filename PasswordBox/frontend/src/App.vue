<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import 'element-plus/dist/index.css'
import {
  CheckInitialized,
  SetupMasterPassword,
  Unlock,
  Lock,
  SavePassword,
  QueryPasswords,
  DeletePassword,
  UpdatePassword,
  SearchPassword,
  ToggleFavorite,
  GetPasswordStrength
} from '../wailsjs/go/main/App'

// ========== 状态 ==========
const mode = ref('loading') // loading | setup | unlock | main
const initialized = ref(false)

// ========== 解锁/初始化表单 ==========
const setupForm = reactive({ password: '', confirm: '' })
const unlockForm = reactive({ password: '' })
const passwordStrength = reactive({ score: 0, level: '', suggestions: [] })

// ========== 密码保存表单 ==========
const saveForm = reactive({ title: '', url: '', username: '', password: '', note: '' })

// ========== 密码列表 ==========
const passwords = ref([])
const searchKeyword = ref('')
const searchLoading = ref(false)
const searchResults = ref([])

// ========== 编辑弹窗 ==========
const editDialogVisible = ref(false)
const editForm = reactive({ id: null, title: '', url: '', username: '', password: '', note: '' })
const editLoading = ref(false)

// ========== 密码显示控制 ==========
const showPwdMap = ref({})

// ========== 初始化检查 ==========
onMounted(async () => {
  try {
    initialized.value = await CheckInitialized()
    mode.value = initialized.value ? 'unlock' : 'setup'
  } catch (e) {
    ElMessage.error('初始化检查失败：' + (e.message || String(e)))
    mode.value = 'setup'
  }
})

// ========== 密码强度检查 ==========
const checkStrength = async (pwd) => {
  if (!pwd) {
    passwordStrength.score = 0
    passwordStrength.level = ''
    passwordStrength.suggestions = []
    return
  }
  const result = await GetPasswordStrength(pwd)
  passwordStrength.score = result.score
  passwordStrength.level = result.level
  passwordStrength.suggestions = result.suggestions
}

// ========== 初始化主密码 ==========
const handleSetup = async () => {
  if (!setupForm.password) {
    ElMessage.warning('请输入主密码')
    return
  }
  if (setupForm.password !== setupForm.confirm) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  try {
    await SetupMasterPassword(setupForm.password)
    ElMessage.success('初始化成功！')
    initialized.value = true
    mode.value = 'main'
    await fetchPasswords()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

// ========== 解锁 ==========
const handleUnlock = async () => {
  if (!unlockForm.password) {
    ElMessage.warning('请输入主密码')
    return
  }
  try {
    await Unlock(unlockForm.password)
    ElMessage.success('解锁成功！')
    mode.value = 'main'
    await fetchPasswords()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

// ========== 锁定 ==========
const handleLock = async () => {
  Lock()
  mode.value = 'unlock'
  unlockForm.password = ''
  passwords.value = []
  searchResults.value = []
  searchKeyword.value = ''
  ElMessage.info('已锁定')
}

// ========== 保存密码 ==========
const handleSave = async () => {
  if (!saveForm.title || !saveForm.password) {
    ElMessage.warning('标题和密码不能为空')
    return
  }
  try {
    await SavePassword(saveForm.title, saveForm.url, saveForm.username, saveForm.password, saveForm.note)
    ElMessage.success('保存成功！')
    saveForm.title = saveForm.url = saveForm.username = saveForm.password = saveForm.note = ''
    await fetchPasswords()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

// ========== 获取全部密码 ==========
const fetchPasswords = async () => {
  try {
    passwords.value = await QueryPasswords()
  } catch (e) {
    passwords.value = []
  }
}

// ========== 搜索 ==========
const handleSearch = async () => {
  if (searchKeyword.value && searchKeyword.value.trim() !== '') {
    searchLoading.value = true
    try {
      searchResults.value = await SearchPassword(searchKeyword.value.trim())
    } catch (e) {
      searchResults.value = []
    }
    searchLoading.value = false
  } else {
    searchResults.value = []
  }
}

// ========== 编辑 ==========
const openEdit = (row) => {
  editForm.id = row.id
  editForm.title = row.title
  editForm.url = row.url
  editForm.username = row.username
  editForm.password = row.password
  editForm.note = row.note
  editDialogVisible.value = true
}

const handleEdit = async () => {
  editLoading.value = true
  try {
    await UpdatePassword(editForm.id, editForm.title, editForm.url, editForm.username, editForm.password, editForm.note)
    ElMessage.success('更新成功！')
    editDialogVisible.value = false
    await fetchPasswords()
    if (searchKeyword.value.trim()) await handleSearch()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  } finally {
    editLoading.value = false
  }
}

// ========== 删除 ==========
const handleDelete = async (row) => {
  ElMessageBox.confirm('确定要删除该密码吗？', '提示', { type: 'warning' })
    .then(async () => {
      await DeletePassword(row.id)
      ElMessage.success('删除成功！')
      await fetchPasswords()
      if (searchKeyword.value.trim()) await handleSearch()
    })
    .catch(() => {})
}

// ========== 收藏 ==========
const handleToggleFavorite = async (row) => {
  try {
    await ToggleFavorite(row.id)
    await fetchPasswords()
    if (searchKeyword.value.trim()) await handleSearch()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

// ========== 复制密码 ==========
const copyPassword = async (pwd) => {
  try {
    await navigator.clipboard.writeText(pwd)
    ElMessage.success('密码已复制到剪贴板，10秒后自动清除')
    setTimeout(async () => {
      const current = await navigator.clipboard.readText()
      if (current === pwd) {
        await navigator.clipboard.writeText('')
      }
    }, 10000)
  } catch (e) {
    ElMessage.error('复制失败：' + (e.message || String(e)))
  }
}

// ========== 切换显示/隐藏 ==========
const toggleShowPwd = (id) => {
  showPwdMap.value[id] = !showPwdMap.value[id]
}
</script>

<template>
  <div class="container">
    <div class="main-gui">
      <!-- 加载中 -->
      <el-card v-if="mode==='loading'" class="box-card center">
        <el-icon class="is-loading"><Loading /></el-icon>
        <div style="margin-top:12px;">加载中...</div>
      </el-card>

      <!-- 初始化主密码 -->
      <el-card v-else-if="mode==='setup'" class="box-card">
        <template #header><span>首次使用 - 设置主密码</span></template>
        <el-form :model="setupForm" label-width="80px">
          <el-form-item label="主密码">
            <el-input
              v-model="setupForm.password"
              type="password"
              placeholder="设置主密码"
              show-password
              clearable
              @input="checkStrength(setupForm.password)"
            />
          </el-form-item>
          <el-form-item>
            <div style="color:#666;font-size:13px;">
              强度: <span :style="{color: passwordStrength.level==='强'?'#67C23A':passwordStrength.level==='中'?'#E6A23C':'#F56C6C'}">{{ passwordStrength.level || '-' }}</span>
              <span v-if="passwordStrength.suggestions.length" style="margin-left:8px;color:#909399;">({{ passwordStrength.suggestions.join('；') }})</span>
            </div>
          </el-form-item>
          <el-form-item label="确认密码">
            <el-input v-model="setupForm.confirm" type="password" placeholder="再次输入主密码" show-password clearable />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleSetup" style="width:100%">初始化</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 解锁 -->
      <el-card v-else-if="mode==='unlock'" class="box-card">
        <template #header><span>解锁 PasswordBox</span></template>
        <el-form :model="unlockForm" label-width="80px">
          <el-form-item label="主密码">
            <el-input v-model="unlockForm.password" type="password" placeholder="请输入主密码" show-password clearable @keyup.enter="handleUnlock" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleUnlock" style="width:100%">解锁</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 主界面 -->
      <el-card v-else class="box-card">
        <template #header>
          <span>密码管理</span>
          <el-button type="danger" size="small" style="float:right;" @click="handleLock">锁定</el-button>
        </template>
        <div style="display: flex; flex-direction: column; gap: 24px;">
          <!-- 搜索 -->
          <div style="background: #f8fafd; border-radius: 8px; padding: 18px 18px 12px 18px; box-shadow: 0 1px 4px #e6e6e6;">
            <div style="margin-bottom: 18px; display: flex; align-items: center; gap: 12px;">
              <el-input
                v-model="searchKeyword"
                placeholder="搜索标题、用户名或网址"
                clearable
                style="max-width: 320px;"
                @input="handleSearch"
                @clear="handleSearch"
                :disabled="searchLoading"
              />
              <el-button :loading="searchLoading" @click="handleSearch" type="primary">搜索</el-button>
            </div>
            <div v-if="searchKeyword && searchKeyword.trim() !== ''" style="margin-bottom: 10px; color: #409EFF; font-size: 15px;">
              <span>搜索"{{ searchKeyword }}"的结果，共 {{ searchResults.length }} 条</span>
            </div>
            <el-table v-if="searchKeyword && searchKeyword.trim() !== ''" :data="searchResults" style="width: 100%;">
              <el-table-column prop="title" label="标题" />
              <el-table-column prop="url" label="网址" />
              <el-table-column prop="username" label="用户名" />
              <el-table-column prop="password" label="密码">
                <template #default="scope">
                  <span v-if="!showPwdMap[scope.row.id]">********</span>
                  <span v-else>{{ scope.row.password }}</span>
                  <el-button size="small" style="margin-left:8px;" @click="toggleShowPwd(scope.row.id)">
                    <span v-if="!showPwdMap[scope.row.id]">显示</span>
                    <span v-else>隐藏</span>
                  </el-button>
                  <el-button size="small" type="primary" style="margin-left:8px;" @click="copyPassword(scope.row.password)">复制</el-button>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180">
                <template #default="scope">
                  <el-button size="small" @click="openEdit(scope.row)">编辑</el-button>
                  <el-button size="small" type="danger" @click="handleDelete(scope.row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- 保存密码 -->
          <div style="background: #fff; border-radius: 8px; padding: 18px 18px 12px 18px; box-shadow: 0 1px 4px #e6e6e6;">
            <el-form :model="saveForm" label-width="80px" class="save-form-vertical">
              <el-row :gutter="12">
                <el-col :xs="24" :sm="12" :md="8" :lg="6">
                  <el-form-item label="标题">
                    <el-input v-model="saveForm.title" placeholder="如 GitHub" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="12" :md="8" :lg="6">
                  <el-form-item label="网址">
                    <el-input v-model="saveForm.url" placeholder="如 https://github.com" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="12" :md="8" :lg="6">
                  <el-form-item label="用户名">
                    <el-input v-model="saveForm.username" placeholder="用户名/邮箱" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="12" :md="8" :lg="6">
                  <el-form-item label="密码">
                    <el-input v-model="saveForm.password" placeholder="密码" show-password clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="24" :md="16" :lg="12">
                  <el-form-item label="备注">
                    <el-input v-model="saveForm.note" placeholder="备注信息" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="24" :md="8" :lg="6">
                  <el-form-item label-width="0">
                    <el-button type="primary" @click="handleSave" style="width:100%">保存</el-button>
                  </el-form-item>
                </el-col>
              </el-row>
            </el-form>

            <!-- 全部密码 -->
            <div style="margin-top: 20px;">
              <div style="font-size: 15px; color: #222; margin-bottom: 8px;">全部密码</div>
              <el-table :data="passwords" style="width: 100%;">
                <el-table-column prop="title" label="标题" />
                <el-table-column prop="url" label="网址" />
                <el-table-column prop="username" label="用户名" />
                <el-table-column prop="password" label="密码">
                  <template #default="scope">
                    <span v-if="!showPwdMap[scope.row.id]">********</span>
                    <span v-else>{{ scope.row.password }}</span>
                    <el-button size="small" style="margin-left:8px;" @click="toggleShowPwd(scope.row.id)">
                      <span v-if="!showPwdMap[scope.row.id]">显示</span>
                      <span v-else>隐藏</span>
                    </el-button>
                    <el-button size="small" type="primary" style="margin-left:8px;" @click="copyPassword(scope.row.password)">复制</el-button>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="220">
                  <template #default="scope">
                    <el-button size="small" @click="openEdit(scope.row)">编辑</el-button>
                    <el-button size="small" type="danger" @click="handleDelete(scope.row)">删除</el-button>
                    <el-button size="small" :type="scope.row.isFavorite ? 'warning' : 'default'" @click="handleToggleFavorite(scope.row)">
                      {{ scope.row.isFavorite ? '取消收藏' : '收藏' }}
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <el-dialog v-model="editDialogVisible" title="编辑密码" width="450px">
              <el-form :model="editForm" label-width="80px">
                <el-form-item label="标题">
                  <el-input v-model="editForm.title" />
                </el-form-item>
                <el-form-item label="网址">
                  <el-input v-model="editForm.url" />
                </el-form-item>
                <el-form-item label="用户名">
                  <el-input v-model="editForm.username" />
                </el-form-item>
                <el-form-item label="密码">
                  <el-input v-model="editForm.password" />
                </el-form-item>
                <el-form-item label="备注">
                  <el-input v-model="editForm.note" />
                </el-form-item>
              </el-form>
              <template #footer>
                <el-button @click="editDialogVisible = false">取消</el-button>
                <el-button type="primary" :loading="editLoading" @click="handleEdit">保存</el-button>
              </template>
            </el-dialog>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<style>
.container {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100vw;
  height: 100vh;
  background: #f5f6fa;
  padding: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.main-gui {
  width: 100vw;
  min-height: 100vh;
  margin: 0;
  padding: 0 0.5vw;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: flex-start;
  box-sizing: border-box;
}
.box-card {
  margin-top: 24px;
  width: 100%;
  box-sizing: border-box;
}
.center {
  text-align: center;
  padding: 40px;
}
.save-form-vertical {
  width: 100%;
  margin-left: 0;
  margin-bottom: 0;
}
</style>
