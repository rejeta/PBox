<script setup>
import { ref, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import 'element-plus/dist/index.css'
import { Register, Login, SavePassword, QueryPasswords, DeletePassword, UpdatePassword, SearchPassword } from '../wailsjs/go/main/App'
// 密码显示控制
const showPwdMap = ref({}) // id: true/false
// 复制密码到剪贴板并10秒后清空
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
// 切换显示/隐藏
const toggleShowPwd = (id) => {
  showPwdMap.value[id] = !showPwdMap.value[id]
}

const mode = ref('login')
const loginForm = reactive({ username: '', password: '' })
const registerForm = reactive({ username: '', password: '' })
const saveForm = reactive({ site: '', account: '', password: '' })
const passwords = ref([]) // 全部密码
const searchKeyword = ref('')
const searchLoading = ref(false)
const searchResults = ref([]) // 搜索结果
const editDialogVisible = ref(false)
const editForm = reactive({ id: null, site: '', account: '', password: '' })
const editLoading = ref(false)
// 打开编辑弹窗
const openEdit = (row, idx) => {
  editForm.id = row.id || idx // 若后端返回有id字段则用id，否则用索引
  editForm.site = row.site
  editForm.account = row.account
  editForm.password = row.password
  editDialogVisible.value = true
}

// 提交编辑
const handleEdit = async () => {
  editLoading.value = true
  try {
    await UpdatePassword(editForm.id, editForm.site, editForm.account, editForm.password)
    ElMessage.success('更新成功！')
    editDialogVisible.value = false
    await fetchPasswords()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  } finally {
    editLoading.value = false
  }
}

// 删除密码
const handleDelete = async (row, idx) => {
  const id = row.id || idx
  ElMessageBox.confirm('确定要删除该密码吗？', '提示', { type: 'warning' })
    .then(async () => {
      await DeletePassword(id)
      ElMessage.success('删除成功！')
      await fetchPasswords()
    })
    .catch(() => {})
}

const handleRegister = async () => {
  try {
    await Register(registerForm.username, registerForm.password)
    ElMessage.success('注册成功，请登录！')
    mode.value = 'login'
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

const handleLogin = async () => {
  try {
    await Login(loginForm.username, loginForm.password)
    mode.value = 'main'
    await fetchPasswords()
    ElMessage.success('登录成功！')
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}

const handleSave = async () => {
  try {
    await SavePassword(saveForm.site, saveForm.account, saveForm.password)
    ElMessage.success('保存成功！')
    saveForm.site = saveForm.account = saveForm.password = ''
    await fetchPasswords()
  } catch (e) {
    ElMessage.error(e.message || String(e))
  }
}


const fetchPasswords = async () => {
  try {
    passwords.value = await QueryPasswords()
  } catch (e) {
    passwords.value = []
  }
}

// 搜索框输入时触发
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

// 搜索框输入时触发（已合并到新版 handleSearch，避免重复声明）

const logout = () => {
  mode.value = 'login'
  loginForm.username = loginForm.password = ''
  passwords.value = []
  ElMessage.info('已退出登录')
}
</script>


<template>
  <div class="container">
    <div class="main-gui">
      <el-card v-if="mode==='login'" class="box-card">
        <template #header><span>登录</span></template>
        <el-form :model="loginForm" label-width="60px">
          <el-form-item label="用户名">
            <el-input v-model="loginForm.username" placeholder="用户名" clearable />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="loginForm.password" type="password" placeholder="密码" show-password clearable />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleLogin" style="width:100%">登录</el-button>
          </el-form-item>
          <el-form-item>
            <span>没有账号？<el-link type="primary" @click="mode='register'">注册</el-link></span>
          </el-form-item>
        </el-form>
      </el-card>
      <el-card v-else-if="mode==='register'" class="box-card">
        <template #header><span>注册</span></template>
        <el-form :model="registerForm" label-width="60px">
          <el-form-item label="用户名">
            <el-input v-model="registerForm.username" placeholder="用户名" clearable />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="registerForm.password" type="password" placeholder="密码" show-password clearable />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleRegister" style="width:100%">注册</el-button>
          </el-form-item>
          <el-form-item>
            <span>已有账号？<el-link type="primary" @click="mode='login'">登录</el-link></span>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card v-else class="box-card">
        <template #header>
          <span>密码管理</span>
          <el-button type="danger" size="small" style="float:right;" @click="logout">退出</el-button>
        </template>
        <div style="display: flex; flex-direction: column; gap: 24px;">
          <!-- 上半部分：搜索及结果 -->
          <div style="background: #f8fafd; border-radius: 8px; padding: 18px 18px 12px 18px; box-shadow: 0 1px 4px #e6e6e6;">
            <div style="margin-bottom: 18px; display: flex; align-items: center; gap: 12px;">
              <el-input
                v-model="searchKeyword"
                placeholder="请输入账号关键字搜索"
                clearable
                style="max-width: 320px;"
                @input="handleSearch"
                @clear="handleSearch"
                :disabled="searchLoading"
              >
                <template #prefix>
                  <i class="el-icon-search" />
                </template>
              </el-input>
              <el-button :loading="searchLoading" @click="handleSearch" type="primary">搜索</el-button>
            </div>
            <div v-if="searchKeyword && searchKeyword.trim() !== ''" style="margin-bottom: 10px; color: #409EFF; font-size: 15px;">
              <span>搜索“{{ searchKeyword }}”的结果，共 {{ searchResults.length }} 条</span>
            </div>
            <el-table v-if="searchKeyword && searchKeyword.trim() !== ''" :data="searchResults" style="width: 100%;">
              <el-table-column prop="site" label="网站/应用" />
              <el-table-column prop="account" label="账号" />
              <el-table-column prop="password" label="密码">
                <template #default="scope">
                  <span v-if="!showPwdMap[scope.row.id]">
                    {{ '********' }}
                  </span>
                  <span v-else>
                    {{ scope.row.password }}
                  </span>
                  <el-button size="small" style="margin-left:8px;" @click="toggleShowPwd(scope.row.id)">
                    <span v-if="!showPwdMap[scope.row.id]">显示</span>
                    <span v-else>隐藏</span>
                  </el-button>
                  <el-button size="small" type="primary" style="margin-left:8px;" @click="copyPassword(scope.row.password)">复制</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <!-- 下半部分：密码保存和全部管理 -->
          <div style="background: #fff; border-radius: 8px; padding: 18px 18px 12px 18px; box-shadow: 0 1px 4px #e6e6e6;">
            <el-form :model="saveForm" label-width="80px" class="save-form-vertical">
              <el-row :gutter="12">
                <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
                  <el-form-item label="网站/应用">
                    <el-input v-model="saveForm.site" placeholder="网站/应用" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
                  <el-form-item label="账号">
                    <el-input v-model="saveForm.account" placeholder="账号" clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="12" :md="8" :lg="6" :xl="6">
                  <el-form-item label="密码">
                    <el-input v-model="saveForm.password" placeholder="密码" show-password clearable />
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :sm="24" :md="24" :lg="6" :xl="6">
                  <el-form-item label-width="0">
                    <el-button type="primary" @click="handleSave" style="width:100%">保存</el-button>
                  </el-form-item>
                </el-col>
              </el-row>
            </el-form>
            <div style="margin-top: 20px;">
              <div style="font-size: 15px; color: #222; margin-bottom: 8px;">全部密码</div>
              <el-table :data="passwords" style="width: 100%;">
                <el-table-column prop="site" label="网站/应用" />
                <el-table-column prop="account" label="账号" />
                <el-table-column prop="password" label="密码">
                  <template #default="scope">
                    <span v-if="!showPwdMap[scope.row.id]">
                      {{ '********' }}
                    </span>
                    <span v-else>
                      {{ scope.row.password }}
                    </span>
                    <el-button size="small" style="margin-left:8px;" @click="toggleShowPwd(scope.row.id)">
                      <span v-if="!showPwdMap[scope.row.id]">显示</span>
                      <span v-else>隐藏</span>
                    </el-button>
                    <el-button size="small" type="primary" style="margin-left:8px;" @click="copyPassword(scope.row.password)">复制</el-button>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="140">
                  <template #default="scope">
                    <el-button size="small" @click="openEdit(scope.row, scope.$index)">编辑</el-button>
                    <el-button size="small" type="danger" @click="handleDelete(scope.row, scope.$index)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <el-dialog v-model="editDialogVisible" title="编辑密码" width="350px">
              <el-form :model="editForm" label-width="80px">
                <el-form-item label="网站/应用">
                  <el-input v-model="editForm.site" />
                </el-form-item>
                <el-form-item label="账号">
                  <el-input v-model="editForm.account" />
                </el-form-item>
                <el-form-item label="密码">
                  <el-input v-model="editForm.password" />
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
.box-card {
  margin-top: 24px;
}
.save-form-vertical {
  width: 100%;
  margin-left: 0;
  margin-bottom: 0;
}
</style>
