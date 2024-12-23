package mailer

import (
	"bytes"
	"embed"
	"github.com/go-mail/mail/v2"
	"html/template"
	"time"
)

// 声明一个embed.FS变量存储email模板，在他上面有一个注释指令表明我们想要存储哪个文件的模板
//
/*
	1. 只能在包级别的变量上使用//go:embed指令，不能在函数或者方法中使用
	2. 路径应该相对于该指令的源代码
	3. 路径不能包含.or..，也不能以/开头或结尾，这实际上限制了你只能嵌入与源代码位于同一目录（或子目录）的文件
	4. 如果路径指向一个目录，那么会递归加载该目录下的所有文件，但是不会加载.  /开头的文件，如果需要加载，需要在路径中使用通配符//go:embed "templates/*"
	5. 可以在一个指令中指定多个目录和文件
*/

//go:embed "templates"
var templateFS embed.FS

// Mailer mail.Dialer用于连接SMTP服务器  email的发送信息，包含名字和地址"Alice Smith <alice@example.com>"
type Mailer struct {
	dialer *mail.Dialer
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	//使用SMTP服务器的配置初始化一个mail.Dialer实例，发送邮件时使用五秒的超时配置
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second

	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

// Send
//
//	@Description:
//	@receiver m
//	@param recipient  接受者邮件地址
//	@param templateFile 模版文件名
//	@param data 模板的动态数据
//	@return error
func (m *Mailer) Send(recipient, templateFile string, data interface{}) error {
	//1. 解析模板 从embed文件系统解析请求模板
	parseFS, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	//2. 执行模版 执行模板文件，将参数传递进去，将结果存储在bytes.Buffer变量中
	subject := new(bytes.Buffer)
	err = parseFS.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}
	plainBody := new(bytes.Buffer)
	err = parseFS.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}
	htmlBody := new(bytes.Buffer)
	err = parseFS.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}
	//设置邮件具体内容
	message := mail.NewMessage()
	message.SetHeader("To", recipient) //设置邮件头信息
	message.SetHeader("From", m.sender)
	message.SetHeader("Subject", subject.String())
	message.SetBody("text/plain", plainBody.String())      //  设置plain-text body  纯文本正文
	message.AddAlternative("text/html", htmlBody.String()) //  设置html body 超文本语言正文

	//打开一个到SMTP服务器的链接，发送信息，关闭链接。如果超时，返回"dial tcp: i/o timeout"
	err = m.dialer.DialAndSend(message)
	if err != nil {
		return err
	}
	return nil

}
