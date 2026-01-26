# 前端项目说明

## 安装依赖

```bash
cd frontend
npm install
```

## 开发运行

```bash
npm run dev
```

访问：http://localhost:5173

## 构建生产版本

```bash
npm run build
```

## 技术栈

- Vue 3.4
- TypeScript
- Vite 5
- Element Plus 2.5
- Tailwind CSS 3.4
- Vue Router 4.2
- Pinia 2.1
- TanStack Query (Vue Query)
- Axios

## 默认登录信息

- 用户名：admin
- 密码：admin123

## API 代理配置

开发环境下，所有 `/api` 请求会代理到 `http://localhost:8080`

## 项目结构

```
frontend/
├── src/
│   ├── api/          # API 接口
│   ├── assets/       # 静态资源
│   ├── components/   # 通用组件
│   ├── router/       # 路由配置
│   ├── stores/       # Pinia 状态管理
│   ├── styles/       # 全局样式
│   ├── views/        # 页面组件
│   ├── App.vue       # 根组件
│   └── main.ts       # 入口文件
├── public/           # 公共资源
├── index.html        # HTML 模板
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```
