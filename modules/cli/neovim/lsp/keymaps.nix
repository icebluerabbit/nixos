{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.keymaps = {
    aspect-name = "cli.neovim.lsp.keymaps";
    includes = [
      den.aspects.cli._.neovim._.plugins._.snacks
      den.aspects.cli._.neovim._.plugins._.snacks._.picker
    ];
    homeManager = _: {
      programs.nixvim.extraConfigLua = ''
        vim.api.nvim_create_autocmd("LspAttach", {
          callback = function(ev)
            for _, key in ipairs({ "grn", "gra", "gri", "grr", "grt", "grx" }) do
              pcall(vim.keymap.del, { "n", "x" }, key, { buffer = ev.buf })
            end
          end,
        })
      '';

      programs.nixvim.lsp.keymaps = [
        {
          key = "K";
          lspBufAction = "hover";
        }
        {
          key = "<leader>k";
          action.__raw = "function() vim.diagnostic.jump({ count=-1, float=true }) end";
        }
        {
          key = "<leader>j";
          action.__raw = "function() vim.diagnostic.jump({ count=1,  float=true }) end";
        }
        {
          key = "<leader>lx";
          action = "<CMD>LspStop<Enter>";
        }
        {
          key = "<leader>ls";
          action = "<CMD>LspStart<Enter>";
        }
        {
          key = "<leader>lr";
          action = "<CMD>LspRestart<Enter>";
        }
        {
          key = "gd";
          action = "<cmd>lua Snacks.picker.lsp_definitions()<cr>";
          options = {
            desc = "Goto Definition";
          };
        }
        {
          key = "gD";
          action = "<cmd>lua Snacks.picker.lsp_declarations()<cr>";
          options = {
            desc = "Goto Declaration";
          };
        }
        {
          key = "gr";
          action = "<cmd>lua Snacks.picker.lsp_references()<cr>";
          options = {
            desc = "References";
            nowait = true;
          };
        }
        {
          key = "gI";
          action = "<cmd>lua Snacks.picker.lsp_implementations()<cr>";
          options = {
            desc = "Goto Implementation";
          };
        }
        {
          key = "gy";
          action = "<cmd>lua Snacks.picker.lsp_type_definitions()<cr>";
          options = {
            desc = "Goto Type Definition";
          };
        }
        {
          key = "gai";
          action = "<cmd>lua Snacks.picker.lsp_incoming_calls()<cr>";
          options = {
            desc = "Calls Incoming";
          };
        }
        {
          key = "gao";
          action = "<cmd>lua Snacks.picker.lsp_outgoing_calls()<cr>";
          options = {
            desc = "Calls Outgoing";
          };
        }
        {
          key = "<leader>ss";
          action = "<cmd>lua Snacks.picker.lsp_symbols()<cr>";
          options = {
            desc = "LSP Symbols";
          };
        }
        {
          key = "<leader>sS";
          action = "<cmd>lua Snacks.picker.lsp_workspace_symbols()<cr>";
          options = {
            desc = "LSP Workspace Symbols";
          };
        }
        {
          key = "<leader>ca";
          action.__raw = "vim.lsp.buf.code_action";
          options.desc = "Code Action";
          mode = [
            "n"
            "x"
          ];
        }
        {
          key = "<leader>cr";
          action.__raw = "vim.lsp.buf.rename";
          options.desc = "Rename";
        }
        {
          key = "<leader>cl";
          action.__raw = "vim.lsp.codelens.run";
          options.desc = "Run Codelens";
        }
      ];
    };
  };
}
