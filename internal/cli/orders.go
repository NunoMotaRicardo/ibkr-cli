package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
)

type orderFlags struct {
	contract, side, qty, typ, price, tif, orderID string
	ex                                            extra
}

func (o *orderFlags) register(c *cobra.Command, withOrderID bool) {
	f := c.Flags()
	f.StringVar(&o.contract, "contract", "", "contract id (the contract_id_ex / contract id the tool expects)")
	f.StringVar(&o.side, "side", "", "BUY or SELL")
	f.StringVar(&o.qty, "qty", "", "quantity")
	f.StringVar(&o.typ, "type", "", "order type, e.g. LMT, MKT")
	f.StringVar(&o.price, "price", "", "limit price")
	f.StringVar(&o.tif, "tif", "", "time in force, e.g. DAY, GTC")
	if withOrderID {
		f.StringVar(&o.orderID, "order-id", "", "order id (from `ibkr orders`)")
	}
	o.ex.register(c)
}

// kv maps the named flags onto whatever parameter names the live schema uses.
func (o *orderFlags) kv(t mcp.Tool) []string {
	var kv []string
	kvAdd(&kv, pk(t, "contract_id_ex", "contract_id", "conid"), o.contract)
	kvAdd(&kv, pk(t, "side", "action"), strings.ToUpper(o.side))
	kvAdd(&kv, pk(t, "quantity", "qty", "size"), o.qty)
	kvAdd(&kv, pk(t, "order_type", "type"), strings.ToUpper(o.typ))
	kvAdd(&kv, pk(t, "price", "limit_price", "lmt_price"), o.price)
	kvAdd(&kv, pk(t, "tif", "time_in_force"), strings.ToUpper(o.tif))
	kvAdd(&kv, pk(t, "order_id", "id"), o.orderID)
	return kv
}

func (o *orderFlags) summary(verb string) string {
	parts := []string{verb}
	for _, p := range [][2]string{{"side", o.side}, {"qty", o.qty}, {"contract", o.contract}, {"type", o.typ}, {"price", o.price}, {"tif", o.tif}, {"order", o.orderID}} {
		if p[1] != "" {
			parts = append(parts, p[0]+"="+p[1])
		}
	}
	return strings.Join(parts, " ")
}

const orderMissing = "Live-order tools only appear when you connected with the orders scope (`ibkr connect --allow-orders`) AND your IBKR account uses the AUTONOMOUS flow (check `ibkr flow`). Under REVIEW_AND_APPROVE use `ibkr instruction create` instead."

func (a *App) instructionCmd() *cobra.Command {
	c := &cobra.Command{Use: "instruction", Short: "Order instructions: reviewable, NOT live orders (create, list, delete)"}
	var of orderFlags
	create := &cobra.Command{Use: "create", Short: "Create an order instruction (needs --yes); prints the review link you must open in IBKR",
		Long: "Creates a reviewable order instruction. Only valid when `ibkr flow` says REVIEW_AND_APPROVE.\nIt is not a live order: the printed link opens IBKR where you review and approve it.\nThe link is ALWAYS printed; show it to the user.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "create_order_instruction", &of.ex, View{}, callOpts{summary: of.summary("create order instruction"), links: "Review link"}, of.kv)
		}}
	of.register(create, false)
	var lex extra
	list := &cobra.Command{Use: "list", Short: "List order instructions", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_order_instructions", &lex, View{ListKeys: lk("instructions", "order_instructions")}, callOpts{links: "Review link"}, nil)
		}}
	lex.register(list)
	var dex extra
	del := &cobra.Command{Use: "delete <id>", Short: "Delete an order instruction", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "delete_order_instruction", &dex, View{}, callOpts{summary: "DELETE order instruction " + args[0]},
				func(t mcp.Tool) []string { return []string{pk(t, "instruction_id", "id") + "=" + args[0]} })
		}}
	dex.register(del)
	c.AddCommand(create, list, del)
	return c
}

func (a *App) orderCmd() *cobra.Command {
	c := &cobra.Command{Use: "order", Short: "LIVE orders (preview, submit, modify, cancel); needs the orders scope + AUTONOMOUS flow",
		Long: "Live-order commands call tools that only exist with the mcp.orders.submit scope and the AUTONOMOUS flow.\nsubmit/modify/cancel need --yes and are never retried after an ambiguous response (exit 5: check `ibkr orders`).\nIn an interactive terminal, preview_order runs first and is shown."}
	mk := func(use, short, tool, verb string, withID bool, noPreview bool) *cobra.Command {
		var of orderFlags
		cc := &cobra.Command{Use: use, Short: short, Args: noArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.exec(cmd, tool, &of.ex, View{}, callOpts{summary: of.summary(verb), noPreview: noPreview, missingHint: orderMissing}, of.kv)
			}}
		of.register(cc, withID)
		return cc
	}
	preview := mk("preview", "Preview an order (read-only)", "preview_order", "preview order", false, true)
	submit := mk("submit", "Submit a LIVE order", "submit_order", "SUBMIT LIVE ORDER", false, false)
	modify := mk("modify", "Modify a LIVE order", "modify_order", "MODIFY LIVE ORDER", true, false)

	var cof orderFlags
	cancel := &cobra.Command{Use: "cancel --order-id <id>", Short: "Cancel a LIVE order", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tc, err := a.toolCache(cmd.Context(), false)
			if err != nil {
				return err
			}
			name := ""
			for _, n := range tc.Names() {
				if strings.HasPrefix(n, "cancel") {
					name = n
					break
				}
			}
			if name == "" {
				if tc, err = a.toolCache(cmd.Context(), true); err == nil {
					for _, n := range tc.Names() {
						if strings.HasPrefix(n, "cancel") {
							name = n
							break
						}
					}
				}
			}
			if name == "" {
				return failErr("no cancel tool is available. %s", orderMissing)
			}
			return a.exec(cmd, name, &cof.ex, View{}, callOpts{summary: cof.summary("CANCEL LIVE ORDER"), noPreview: true, missingHint: orderMissing}, cof.kv)
		}}
	cof.register(cancel, true)
	c.AddCommand(preview, submit, modify, cancel)
	return c
}
