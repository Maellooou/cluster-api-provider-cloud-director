/*
 * VMware Cloud Director OpenAPI
 *
 * VMware Cloud Director OpenAPI is a new API that is defined using the OpenAPI standards.<br/> This ReSTful API borrows some elements of the legacy VMware Cloud Director API and establishes new patterns for use as described below. <h4>Authentication</h4> Authentication and Authorization schemes are the same as those for the legacy APIs. You can authenticate using the JWT token via the <code>Authorization</code> header or specifying a session using <code>x-vcloud-authorization</code> (The latter form is deprecated). <h4>Operation Patterns</h4> This API follows the following general guidelines to establish a consistent CRUD pattern: <table> <tr>   <th>Operation</th><th>Description</th><th>Response Code</th><th>Response Content</th> </tr><tr>   <td>GET /items<td>Returns a paginated list of items<td>200<td>Response will include Navigational links to the items in the list. </tr><tr>   <td>POST /items<td>Returns newly created item<td>201<td>Content-Location header links to the newly created item </tr><tr>   <td>GET /items/urn<td>Returns an individual item<td>200<td>A single item using same data type as that included in list above </tr><tr>   <td>PUT /items/urn<td>Updates an individual item<td>200<td>Updated view of the item is returned </tr><tr>   <td>DELETE /items/urn<td>Deletes the item<td>204<td>No content is returned. </tr> </table> <h5>Asynchronous operations</h5> Asynchronous operations are determined by the server. In those cases, instead of responding as described above, the server responds with an HTTP Response code 202 and an empty body. The tracking task (which is the same task as all legacy API operations use) is linked via the URI provided in the <code>Location</code> header.<br/> All API calls can choose to service a request asynchronously or synchronously as determined by the server upon interpreting the request. Operations that choose to exhibit this dual behavior will have both options documented by specifying both response code(s) below. The caller must be prepared to handle responses to such API calls by inspecting the HTTP Response code. <h5>Error Conditions</h5> <b>All</b> operations report errors using the following error reporting rules: <ul>   <li>400: Bad Request - In event of bad request due to incorrect data or other user error</li>   <li>401: Bad Request - If user is unauthenticated or their session has expired</li>   <li>403: Forbidden - If the user is not authorized or the entity does not exist</li> </ul> <h4>OpenAPI Design Concepts and Principles</h4> <ul>   <li>IDs are full Uniform Resource Names (URNs).</li>   <li>OpenAPI's <code>Content-Type</code> is always <code>application/json</code></li>   <li>REST links are in the Link header.</li>   <ul>     <li>Multiple relationships for any link are represented by multiple values in a space-separated list.</li>     <li>Links have a custom VMware Cloud Director-specific &quot;model&quot; attribute that hints at the applicable data         type for the links.</li>     <li>title + rel + model attributes evaluates to a unique link.</li>     <li>Links follow Hypermedia as the Engine of Application State (HATEOAS) principles. Links are present if         certain operations are present and permitted for the user&quot;s current role and the state of the         referred entities.</li>   </ul>   <li>APIs follow a flat structure relying on cross-referencing other entities instead of the navigational style       used by the legacy VMware Cloud Director APIs.</li>   <li>Most endpoints that return a list support filtering and sorting similar to the query service in the legacy       VMware Cloud Director APIs.</li>   <li>Accept header must be included to specify the API version for the request similar to calls to existing legacy       VMware Cloud Director APIs.</li>   <li>Each feature has a version in the path element present in its URL.<br/>       <b>Note</b> API URL's without a version in their paths must be considered experimental.</li> </ul>
 *
 * API version: 36.0
 * Contact: https://code.vmware.com/support
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package swagger

// The configuration for a given NAT Rule.
type EdgeNatRule struct {
	// The unique id of the NAT Rule. This must be supplied when updating a given NAT Rule. On creation, an unique id is generated for the NAT Rule.
	Id string `json:"id,omitempty"`
	// User friendly name for the NAT Rule. Name must be provided.
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// A flag indicating whether the individual nat rule is enabled or not. The default is true.
	Enabled bool `json:"enabled,omitempty"`
	// Represents the type of NAT Rule. SNAT translates an internal IP to an external IP and is used for outbound traffic. DNAT translates the external IP to an internal IP and is used for inbound traffic. This property is now deprecated and replaced with type.
	RuleType *NatRuleType `json:"ruleType,omitempty"`
	// Represents the type of NAT Rule.  Below are valid values. <ul>   <li> <code> SNAT </code> - This translates an internal IP to an external IP and is used for outbound traffic.   <li> <code> DNAT </code> - This translates the external IP to an internal IP and is used for inbound traffic.   <li> <code> NO_SNAT </code> - No internal IP translation takes place.   <li> <code> NO_DNAT </code> - No external IP translation takes place.   <li> <code> REFLEXIVE </code> - Also known as Stateless NAT. This translates an internal IP to an external IP and vice versa.   The number of internal addresses should be exactly the same as that of external addresses. </ul>
	Type_ string `json:"type,omitempty"`
	// Represents the application ports on which the NAT Rule will be applied. An application port profile id in the form of URN format must be provided. If not provided then the port will be considered as \"ANY\". This should not be set for a REFLEXIVE Rule. For a DNAT Rule, the source port on the application port profile represents the port from which the traffic is originating from. For a DNAT rule, the destination port on the application port profile represents the internal port on the workloads where the traffic is terminating. For a SNAT rule, the source port on the application port profile represents the internal port on the workloads where the traffic is originating from. For a SNAT rule, the destination port application port profile represents the port where the traffic is terminating.
	ApplicationPortProfile *EntityReference `json:"applicationPortProfile,omitempty"`
	// The external addresses for the NAT Rule. This must be supplied as a single IP or Network CIDR. For a DNAT rule, this is the external facing IP Address for incoming traffic. For an SNAT rule, this is the external facing IP Address for outgoing traffic. These ips are typically allocated/suballocated IP Addresses on the Edge Gateway. For a REFLEXIVE rule, these are the external facing IPs.
	ExternalAddresses string `json:"externalAddresses"`
	// The internal addresses for the NAT Rule. This must be supplied as a single IP or Network CIDR. For a DNAT rule, this is the internal IP Address for incoming traffic. For an SNAT rule, this is the internal IP Address for outgoing traffic. For a REFLEXIVE rule, these are the internal IPs. These ips are typically the Private IPs that are allocated to workloads.
	InternalAddresses string `json:"internalAddresses"`
	// Port number or port range for incoming network traffic. If Any Traffic is selected for the Service, the default internal port is \"ANY\". Note that this field has been deprecated. Please use dnatExternalPort to set port forwarding for DNAT rules. This typically should not be set for SNAT rules as the rule would not be able to support IP Translation with multiple ports.
	InternalPort string `json:"internalPort,omitempty"`
	// This represents the external port number or port range when doing DNAT port forwarding from external to internal. The default dnatExternalPort is \"ANY\" meaning traffic on any port for the given IPs selected will be translated.
	DnatExternalPort string `json:"dnatExternalPort,omitempty"`
	// A flag indicating whether logging for the individual nat rule is enabled or not. The default is false.
	Logging bool `json:"logging,omitempty"`
	// A flag indicating whether this NAT rule is managed by the system. This is not user editable
	SystemRule bool `json:"systemRule,omitempty"`
	// The destination addresses to match in the SNAT Rule. This must be supplied as a single IP or Network CIDR. Providing no value for this field results in match with ANY destination network. These IPs are typically the Private IPs that are allocated to destination workloads.
	SnatDestinationAddresses string `json:"snatDestinationAddresses,omitempty"`
	// Determines how the firewall matches the address during NATing if firewall stage is not skipped.  Below are valid values. <ul>   <li> <code> MATCH_INTERNAL_ADDRESS </code> indicates the firewall will be applied to internal address of a NAT rule. For SNAT, the internal address is        the original source address before NAT is done. For DNAT, the internal address is the translated destination address after NAT is done.        For REFLEXIVE, to egress traffic, the internal address is the original source address before NAT is done; to ingress traffic, the internal address is        the translated destination address after NAT is done.   <li> <code> MATCH_EXTERNAL_ADDRESS </code> indicates the firewall will be applied to external address of a NAT rule. For SNAT, the external address is        the translated source address after NAT is done. For DNAT, the external address is the original destination address before NAT is done.        For REFLEXIVE, to egress traffic, the external address is the translated internal address after NAT is done; to ingress traffic, the external address is        the original destination address before NAT is done.   <li> <code> BYPASS </code> firewall stage will be skipped. </ul> The default is MATCH_INTERNAL_ADDRESS.
	FirewallMatch string `json:"firewallMatch,omitempty"`
	// If an address has multiple NAT rules, the rule with the highest priority is applied. A lower value means a higher precedence for this rule.
	Priority int32          `json:"priority,omitempty"`
	Version  *ObjectVersion `json:"version,omitempty"`
}
