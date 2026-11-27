package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sya-ri/tovela/internal/locale"
)

type apiError struct {
	Code    int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

func isHTTP(err error, status int) bool {
	var response *apiError
	return errors.As(err, &response) && response.Code == status
}

func request(ctx context.Context, client *http.Client, method, address string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		var fields struct {
			Error  struct{ Message string }
			Errors []struct{ Message string }
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&fields)
		message := fields.Error.Message
		if message == "" && len(fields.Errors) > 0 {
			message = fields.Errors[0].Message
		}
		return &apiError{Code: res.StatusCode, Message: message}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out)
}

type compute struct {
	client *http.Client
	config Config
}

type instance struct {
	ID                string
	Status            string
	Labels            map[string]string
	CreationTimestamp time.Time
	Scheduling        struct {
		TerminationTime      time.Time
		TerminationTimestamp time.Time
	}
	NetworkInterfaces []struct {
		AccessConfigs []struct{ NatIP string }
	}
}

func (i *instance) ip() string {
	for _, nic := range i.NetworkInterfaces {
		for _, config := range nic.AccessConfigs {
			if ip := net.ParseIP(config.NatIP); ip != nil && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return ""
}

func (i *instance) expiry() time.Time {
	if !i.Scheduling.TerminationTimestamp.IsZero() {
		return i.Scheduling.TerminationTimestamp
	}
	return i.Scheduling.TerminationTime
}

type operation struct {
	Name   string
	Status string
	Error  struct {
		Errors []struct{ Code, Message string }
	}
}

func (c compute) projectEndpoint() string {
	return "https://compute.googleapis.com/compute/v1/projects/" + c.config.Project
}

func (c compute) endpoint() string {
	return c.projectEndpoint() + "/zones/" + c.config.Zone
}

func (c compute) lookup(ctx context.Context) (*instance, error) {
	var vm instance
	err := request(ctx, c.client, http.MethodGet, c.endpoint()+"/instances/"+c.config.name(), nil, &vm)
	if isHTTP(err, 404) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func pause(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c compute) wait(ctx context.Context, op operation) error {
	for op.Status != "DONE" {
		if op.Name == "" {
			return errors.New(locale.T("GCE did not return an operation ID"))
		}
		if err := pause(ctx, 2*time.Second); err != nil {
			return err
		}
		if err := request(ctx, c.client, http.MethodGet, c.endpoint()+"/operations/"+url.PathEscape(op.Name), nil, &op); err != nil && !isHTTP(err, 404) {
			return err
		}
	}
	if len(op.Error.Errors) > 0 {
		failure := op.Error.Errors[0]
		if failure.Code == "RESOURCE_ALREADY_EXISTS" {
			return &apiError{Code: 409, Message: failure.Message}
		}
		return fmt.Errorf("GCE: %s (%s)", failure.Message, failure.Code)
	}
	return nil
}

func (c compute) checkMachineType(ctx context.Context) error {
	err := request(ctx, c.client, http.MethodGet, c.endpoint()+"/machineTypes/"+c.config.RelayMachineType(), nil, nil)
	if isHTTP(err, http.StatusNotFound) {
		return fmt.Errorf(locale.T("Google Cloud could not find %s in zone %s."), c.config.RelayMachineType(), c.config.Zone)
	}
	if err != nil {
		return fmt.Errorf(locale.T("Could not check Google Cloud zone %s: %w"), c.config.Zone, err)
	}
	return nil
}

func (c compute) create(ctx context.Context, session identity) error {
	payload := map[string]any{
		"name":            c.config.name(),
		"machineType":     "zones/" + c.config.Zone + "/machineTypes/" + c.config.RelayMachineType(),
		"labels":          map[string]string{"managed-by": "tovela", "relay": c.config.name(), "session": session.Nonce},
		"serviceAccounts": []any{},
		"tags":            map[string]any{"items": []string{"tovela"}},
		"disks": []any{map[string]any{
			"boot": true, "autoDelete": true,
			"initializeParams": map[string]string{"diskType": "zones/" + c.config.Zone + "/diskTypes/pd-standard", "diskSizeGb": "10", "sourceImage": "projects/debian-cloud/global/images/family/debian-13"},
		}},
		"networkInterfaces": []any{map[string]any{
			"network":       "global/networks/" + c.config.Network,
			"accessConfigs": []any{map[string]string{"name": "External NAT", "type": "ONE_TO_ONE_NAT"}},
		}},
		"metadata": map[string]any{"items": []map[string]string{{"key": "startup-script", "value": serverStartup(session, c.config.Ports)}}},
		"scheduling": map[string]any{
			"terminationTime": session.Deadline.Format(time.RFC3339), "instanceTerminationAction": "DELETE",
		},
	}
	var op operation
	endpoint := c.endpoint() + "/instances?requestId=" + session.RequestID
	var err error
	for retry := 0; retry < 2; retry++ {
		err = request(ctx, c.client, http.MethodPost, endpoint, payload, &op)
		var response *apiError
		if err == nil || (errors.As(err, &response) && response.Code < 500 && response.Code != 429) {
			break
		}
		if waitErr := pause(ctx, time.Second); waitErr != nil {
			return waitErr
		}
	}
	if err != nil {
		return err
	}
	return c.wait(ctx, op)
}

func (c compute) extend(ctx context.Context, session identity) (time.Time, error) {
	vm, err := c.lookup(ctx)
	if err != nil {
		return time.Time{}, err
	}
	if vm == nil || vm.Status != "RUNNING" || !c.owns(vm, session) || vm.ID != session.InstanceID {
		return time.Time{}, errors.New(locale.T("The running VM does not match this session."))
	}
	current := vm.expiry()
	if current.IsZero() || !time.Now().Before(current) {
		return time.Time{}, errors.New(locale.T("The relay has already expired."))
	}
	deadline := current.Add(extension).UTC().Truncate(time.Second)
	if deadline.After(vm.CreationTimestamp.Add(120 * 24 * time.Hour)) {
		return time.Time{}, errors.New(locale.T("The extension exceeds the maximum Google Cloud VM runtime."))
	}
	payload := map[string]string{
		"terminationTime": deadline.Format(time.RFC3339), "instanceTerminationAction": "DELETE",
	}
	var op operation
	endpoint := c.endpoint() + "/instances/" + c.config.name() + "/setScheduling?requestId=" + requestID()
	if err := request(ctx, c.client, http.MethodPost, endpoint, payload, &op); err != nil {
		return time.Time{}, err
	}
	if err := c.wait(ctx, op); err != nil {
		return time.Time{}, err
	}
	return deadline, nil
}

func (c compute) delete(ctx context.Context, session identity) error {
	vm, err := c.lookup(ctx)
	if err != nil || vm == nil {
		return err
	}
	if !c.owns(vm, session) || (session.InstanceID != "" && vm.ID != session.InstanceID) {
		return errors.New(locale.T("The VM does not match the session that this app created."))
	}
	var op operation
	err = request(ctx, c.client, http.MethodDelete, c.endpoint()+"/instances/"+c.config.name()+"?requestId="+requestID(), nil, &op)
	if isHTTP(err, 404) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.wait(ctx, op)
}

func (c compute) owns(vm *instance, session identity) bool {
	return session.Nonce != "" && vm.Labels["managed-by"] == "tovela" &&
		vm.Labels["relay"] == c.config.name() && vm.Labels["session"] == session.Nonce
}

const cfEndpoint = "https://api.cloudflare.com/client/v4"

type dnsRecord struct {
	ID      string
	Type    string
	Name    string
	Content string
	Comment string
}

// DNSConflict reports a hostname with records that require replacement consent.
type DNSConflict struct {
	Hostname string
}

func (e *DNSConflict) Error() string {
	return fmt.Sprintf(locale.T("Existing DNS records for %s."), e.Hostname)
}

type cloudflare struct {
	client *http.Client
	config Config
}

func (c cloudflare) zone(ctx context.Context) (string, error) {
	explicit := c.config.CloudflareZone
	zone := explicit
	if zone == "" {
		zone = c.config.Domain
	}
	for strings.Contains(zone, ".") {
		var response struct{ Result []struct{ ID string } }
		err := request(ctx, c.client, http.MethodGet, cfEndpoint+"/zones?name="+url.QueryEscape(zone), nil, &response)
		if err != nil {
			return "", err
		}
		switch len(response.Result) {
		case 1:
			return response.Result[0].ID, nil
		case 0:
			if explicit != "" {
				return "", fmt.Errorf(locale.T("Cloudflare zone %s was not found"), zone)
			}
		default:
			return "", fmt.Errorf(locale.T("Cloudflare zone %s is ambiguous"), zone)
		}
		_, zone, _ = strings.Cut(zone, ".")
	}
	return "", fmt.Errorf(locale.T("No Cloudflare zone found for %s"), c.config.Domain)
}

func (c cloudflare) records(ctx context.Context, root, name string) ([]dnsRecord, error) {
	var response struct{ Result []dnsRecord }
	err := request(ctx, c.client, http.MethodGet, root+"?name="+url.QueryEscape(name), nil, &response)
	return response.Result, err
}

func (c cloudflare) recordOwner(session identity) string {
	return "tovela " + c.config.name() + " " + session.Nonce
}

func (c cloudflare) ensureAvailable(ctx context.Context, root string, session identity, replace bool) ([]map[string]string, error) {
	var deletes []map[string]string
	names := []string{c.config.Domain}
	if slices.ContainsFunc(c.config.Ports, func(port PortMapping) bool { return port.Protocol == MinecraftJava }) {
		names = append(names, "_minecraft._tcp."+c.config.Domain)
	}
	for _, name := range names {
		records, err := c.records(ctx, root, name)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if name == c.config.Domain && record.Type != "A" && record.Type != "AAAA" && record.Type != "CNAME" {
				continue
			}
			if name != c.config.Domain && record.Type != "SRV" {
				continue
			}
			if record.Comment != c.recordOwner(session) && !replace {
				return nil, &DNSConflict{Hostname: c.config.Domain}
			}
			deletes = append(deletes, map[string]string{"id": record.ID})
		}
	}
	return deletes, nil
}

func (c cloudflare) publish(ctx context.Context, zoneID string, session identity, ip string, replace bool) error {
	root := cfEndpoint + "/zones/" + zoneID + "/dns_records"
	deletes, err := c.ensureAvailable(ctx, root, session, replace)
	if err != nil {
		return err
	}
	post := map[string]any{"type": "A", "name": c.config.Domain, "content": ip, "ttl": 60, "proxied": false, "comment": c.recordOwner(session)}
	batch := map[string]any{"posts": []any{post}}
	if len(deletes) != 0 {
		batch["deletes"] = deletes
	}
	var result struct{ Success bool }
	if err := request(ctx, c.client, http.MethodPost, root+"/batch", batch, &result); err != nil {
		return err
	}
	if !result.Success {
		return errors.New(locale.T("Cloudflare did not confirm the DNS update"))
	}
	return nil
}

func (c cloudflare) unpublish(ctx context.Context, zoneID string, session identity) error {
	root := cfEndpoint + "/zones/" + zoneID + "/dns_records"
	records, err := c.records(ctx, root, c.config.Domain)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Type != "A" || record.Comment != c.recordOwner(session) ||
			(session.PublicIP != "" && strings.TrimSpace(record.Content) != session.PublicIP) {
			continue
		}
		err := request(ctx, c.client, http.MethodDelete, root+"/"+url.PathEscape(record.ID), nil, nil)
		if err != nil && !isHTTP(err, 404) {
			return err
		}
	}
	return nil
}
