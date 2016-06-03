package appgroup

import (
	"fmt"
	"strings"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

func pathLogin(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "login$",
		Fields: map[string]*framework.FieldSchema{
			"selector_id": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Identifier of the category the SecretID belongs to.",
			},
			"secret_id": &framework.FieldSchema{
				Type:        framework.TypeString,
				Default:     "",
				Description: "SecretID of the App.",
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation: b.pathLoginUpdate,
		},
		HelpSynopsis:    pathLoginHelpSys,
		HelpDescription: pathLoginHelpDesc,
	}
}

func (b *backend) pathLoginUpdate(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	secretID := strings.TrimSpace(data.Get("secret_id").(string))
	if secretID == "" {
		return logical.ErrorResponse("missing secret_id"), nil
	}

	// Selector can optionally be prepended to the SecretID with a `;` delimiter
	selectorID := strings.TrimSpace(data.Get("selector_id").(string))
	if selectorID == "" {
		selectorFields := strings.SplitN(secretID, ";", 2)
		if len(selectorFields) != 2 || selectorFields[0] == "" {
			return logical.ErrorResponse("missing selector_id"), nil
		} else if selectorFields[1] == "" {
			return logical.ErrorResponse("missing secret_id"), nil
		} else {
			selectorID = selectorFields[0]
			secretID = selectorFields[1]
		}
	}

	validateResp, err := b.validateCredentials(req.Storage, selectorID, secretID)
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to validate secret ID: %s", err)), nil
	}

	resp := &logical.Response{
		Auth: &logical.Auth{
			InternalData: map[string]interface{}{
				"selector_id": validateResp.SelectorID,
			},
			Policies: validateResp.Policies,
			LeaseOptions: logical.LeaseOptions{
				TTL:       validateResp.TokenTTL,
				Renewable: true,
			},
		},
	}
	return resp, nil
}

func (b *backend) pathLoginRenew(req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	selectorID := req.Auth.InternalData["selector_id"].(string)
	if selectorID == "" {
		return nil, fmt.Errorf("failed to fetch selector_id during renewal")
	}

	resp, err := b.validateSelectorID(req.Storage, selectorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate selector during renewal:%s", err)
	}

	return framework.LeaseExtend(resp.TokenTTL, resp.TokenMaxTTL, b.System())(req, data)
}

const pathLoginHelpSys = "Issue a token for a given pair of 'selector' and 'secret_id'."

const pathLoginHelpDesc = `The supplied SecretID could've been generated/assigned against an
individual App, or a Group or a 'supergroup' combination of both.
The respective 'selector' for these categories of SecretIDs are
'app/<app_name>', 'group/<group_name>' or 'supergroup'. The supplied
credentials <'selector','secret_id'> are validated and a Vault token
is issued with effective capabilities to access the participating
Apps.
`