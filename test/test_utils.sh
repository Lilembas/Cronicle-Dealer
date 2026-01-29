#!/bin/bash

# 测试工具库 - 包含所有测试脚本的公共函数

# 获取脚本所在目录的绝对路径
get_script_dir() {
    echo "$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
}

# 获取项目根目录
get_project_root() {
    local script_dir
    script_dir="$(get_script_dir)"
    echo "$(dirname "$script_dir")"
}

# 检查配置文件是否存在
check_config_file() {
    local project_root
    project_root="$(get_project_root)"
    local config_path="$project_root/config.yaml"

    if [ ! -f "$config_path" ]; then
        echo "❌ 配置文件 $config_path 不存在"
        echo "请先复制 config.example.yaml 到 config.yaml"
        exit 1
    fi

    echo "$config_path"
}

# 查找 Go 可执行文件
find_go_binary() {
    local go_bin=""

    if command -v go &> /dev/null; then
        go_bin="go"
    elif [ -x "/usr/local/go/bin/go" ]; then
        go_bin="/usr/local/go/bin/go"
    elif [ -x "/usr/bin/go" ]; then
        go_bin="/usr/bin/go"
    elif [ -x "$HOME/go/bin/go" ]; then
        go_bin="$HOME/go/bin/go"
    else
        echo "❌ Go 未安装或不在 PATH 中" >&2
        echo "尝试的路径: /usr/local/go/bin/go, /usr/bin/go, \$HOME/go/bin/go" >&2
        echo "或设置 PATH: export PATH=\$PATH:/usr/local/go/bin" >&2
        exit 1
    fi

    echo "$go_bin"
}

# 构建测试程序
build_test_program() {
    local go_bin="$1"
    local source_file="$2"
    local output_file="$3"
    local script_dir
    script_dir="$(get_script_dir)"

    cd "$script_dir"
    $go_bin build -o "$output_file" "$source_file"
}

# 运行测试程序
run_test_program() {
    local program="$1"
    shift
    "./$program" "$@"
}

# 清理测试文件
cleanup_test_files() {
    rm -f "$@"
}
