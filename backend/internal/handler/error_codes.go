package handler

// 错误码表：后端只回码，文案在前端（frontend/src/i18n/errors.ts）按中英各写一份，
// 数据库里也不存多语内容——界面用什么语言是前端的事。
// 每条响应里的 error 字段是英文兜底文案，只用来排查问题，不参与界面渲染。
//
// 约束：
//  1. 码一旦对外就是接口契约，新增可以，改名等于破坏兼容；
//  2. 按领域分组，前缀表明来源，比如 auth_ / attachment_ / task_；
//  3. 一个含义只有一条，别为「密码不对」和「密码错误」各建一条。
const (
	// 通用
	codeUnauthorized  = "unauthorized"   // 没登录 / token 失效
	codeInternal      = "internal_error" // 兜底：服务端自己出错了
	codeNotFound      = "not_found"      // 记录不存在，或不属于当前用户
	codeInvalidParams = "invalid_params" // 参数格式不对
	codeInvalidID     = "invalid_id"     // 路径上的 id 不合法
	codeMissingField  = "missing_field"  // 必填项缺了
	codeReloadFailed  = "reload_failed"  // 写成功了，但回读失败

	// 账号与登录
	codeEmailRequired     = "auth_email_required"
	codeEmailInvalid      = "auth_email_invalid"
	codeEmailTaken        = "auth_email_taken"
	codePasswordTooShort  = "auth_password_too_short"
	codePasswordSame      = "auth_password_same"
	codePasswordWrong     = "auth_password_wrong"
	codeCodeWrong         = "auth_code_wrong"
	codeCodeTooFrequent   = "auth_code_too_frequent"
	codeNicknameTooLong   = "auth_nickname_too_long"
	codeMailNotConfigured = "auth_mail_not_configured"
	codeMailSendFailed    = "auth_mail_send_failed"

	// 附件
	codeFileRequired     = "attachment_file_required"
	codeFileTooLarge     = "attachment_file_too_large"
	codeFileTypeRejected = "attachment_file_type_rejected"
	codeFileReadFailed   = "attachment_read_failed"
	codeFileSaveFailed   = "attachment_save_failed"

	// 业务
	codeProjectNotFound     = "project_not_found"
	codeProjectNameTaken    = "project_name_taken"
	codeLabelNameTaken      = "label_name_taken"
	codeLabelNameRequired   = "label_name_required"
	codeTaskNotFound        = "task_not_found"
	codeTaskTitleRequired   = "task_title_required"
	codeIdeaNotFound        = "idea_not_found"
	codeIdeaContentRequired = "idea_content_required"
	codeCommentRequired     = "comment_content_required"
	codeAmountInvalid       = "amount_invalid"
	codeDateRangeRequired   = "date_range_required"
)
