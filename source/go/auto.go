package main;
 
import (
    "github.com/fsnotify/fsnotify"
    "fmt"
    "path/filepath"
	"os"
	"os/exec"
	"bytes"
	"time"
	"strings"
	"errors"
)
 
type Watch struct {
    watch *fsnotify.Watcher;
}

var (
	_look bool
	_i int // 成功次数
	_n int // 失败次数

	_current_cmd[] string
	_error string
	_fatal bool // 致命错误 (需要手动解决冲突)
)
 
//监控目录
func (w *Watch) watchDir(dir string) {
    //通过Walk来遍历目录下的所有子目录
    filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        //这里判断是否为目录，只需监控目录即可
        if info.IsDir() {
			path, err := filepath.Abs(path);
            if err != nil {
                return err;
			}
			
			if (strings.Contains(path, "/.git") ==true || strings.Contains(path, "/.vscode")) {
				return nil;
			}
			
            err = w.watch.Add(path);
            if err != nil {
                return err;
            }
            // fmt.Println("监控 : ", path);
        }
        return nil;
    });
    go func() {
        for {
            select {
            case ev := <-w.watch.Events:
                {
                    if ev.Op&fsnotify.Create == fsnotify.Create {
                        //这里获取新创建文件的信息，如果是目录，则加入监控中
                        fi, err := os.Stat(ev.Name);
                        if err == nil && fi.IsDir() {
							w.watch.Add(ev.Name);
							break; // 空目录没必要推送
                        }
                    }
                    if ev.Op&fsnotify.Remove == fsnotify.Remove {
                        //如果删除文件是目录，则移除监控
                        fi, err := os.Stat(ev.Name);
                        if err == nil && fi.IsDir() {
                            w.watch.Remove(ev.Name);
                        }
                    }
                    if ev.Op&fsnotify.Rename == fsnotify.Rename {
                        fmt.Println("重命名文件 : ", ev.Name);
                        //如果重命名文件是目录，则移除监控
                        //注意这里无法使用os.Stat来判断是否是目录了
                        //因为重命名后，go已经无法找到原文件来获取信息了
                        //所以这里就简单粗爆的直接remove好了
                        w.watch.Remove(ev.Name);
                    }
                    if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
                        break;
					}
					

					// 简单锁
					if (_look) {
						break;
					} else {
						_look = true
					}
					
					bool, out := git([]string{"status", "-s"})
					if (bool==false || out == "") {
						_look = false
						break
					}

					bool, out = git([]string{"add", "."})
					if (bool==false) {
						godie(out, true)
						break
					}

					bool, out = git([]string{"commit", "-m", time.Now().Format("2006-01-02 15:04:05")})
					if (bool==false) {
						godie(out, true)
						break
					}

					bool, out = git([]string{"pull", "origin", "master"})
					if (bool==false) {
						godie(out, true)
						break
					}

					bool, out = git([]string{"push", "origin", "master"})
					if (bool==false) {
						godie(out, true)
						break
					}

					// 推送成功
					_i++;
					_look = false
                }
            case err := <-w.watch.Errors:
                {
                    fmt.Println(err)
                    return;
                }
            }
        }
    }();
}
 


func main() {

	// 当前是否在git工作目录
	// if (!IsDir("./.git")) {
	// 	fmt.Println("这不是一个有效的git工作目录!");
	// 	return;
	// }



	go showMessage();

	watch, _ := fsnotify.NewWatcher()
    w := Watch{
        watch: watch,
    }
    w.watchDir(".");
	select {};
}

func showMessage() {
	loading := "\\";

	for !_fatal {
		// fmt.Println("\033[H\033[2J")
		fmt.Printf("\033[%dA\033[K", 2)

		// 是否在提交
		if (_look) {
			switch loading {
			case "\\":
				loading = "/"
				break
			case "/":
				loading = "-"
				break
			case "-":
				loading = "\\"
				break
			}

			fmt.Println("状态:", "推送中")
			fmt.Println("执行: ", _current_cmd, "[ "+loading+" ]")
			if (_error != "") {
				fmt.Println("输出: ", _error)
			}
			
		} else {
			fmt.Println("状态:", "正常")
			fmt.Println("统计:", "成功", _i, "次, 失败", _n, "次")
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func godie(out string, fatal bool) {
	_error = out
	_fatal = fatal
	_n++

	var input string
	fmt.Println("错误:", out)
	fmt.Println("警告:","请手动解决合并冲突后按回车继续监听")
	fmt.Scanln(&input)

	_fatal = false
	_error = ""
	go showMessage()
}


func git(params []string) (bool, string) {
	bout := bytes.NewBuffer(nil)
	berr := bytes.NewBuffer(nil)
	
	cmd := exec.Command("git", params...)
	cmd.Stdout = bout
	cmd.Stderr = berr
	err := cmd.Run()

	_current_cmd = cmd.Args

	if err != nil {
		_look = false;
		_error = berr.String();
		return false, _error;
	}

	if (bout.String() != "") {
		//fmt.Println(bout.String());
	}
	return true, bout.String();
}

// 获取当前目录
func GetCurrentPath() (string, error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		i = strings.LastIndex(path, "\\")
	}
	if i < 0 {
		return "", errors.New(`error: Can't find "/" or "\".`)
	}
	return string(path[0 : i+1]), nil
}

// 判断所给路径是否为文件夹
func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}


// 基于 http://www.cppblog.com/kenkao/archive/2018/07/31/215809.html 的子目录监控代码完成
// 特别想要git自动提交工具, 所以写了这个程序.  第一次写go  google查了很多才算憋出来了.   
// 欢迎指正代码中的错误和参与完善.  小团队用git真的糟心.  提交信息对我们没卵用,  只是希望保存后可以马上看到效果. 
