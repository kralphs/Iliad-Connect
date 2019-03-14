package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"

	"iliad-connect/parser"
)

func collectFiles(link string) ([]pdf, error) {
	files := []pdf{}

	client := &http.Client{}

	resp, err := client.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Single file case
	if resp.Header.Get("Content-Type") == "application/pdf" {
		disposition := strings.SplitAfter(resp.Header.Get("Content-Disposition"), "filename=")
		name := strings.Trim(disposition[1], "\"")
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		files = append(files, pdf{Name: name, Content: file, URL: link})
		return files, nil
	}

	// Multiple files or invalid link
	urls := parser.GetUrls(resp.Body, false)
	if len(urls) == 0 {
		return nil, errors.New("Invalid or stale link")
	}

	// Convert relative paths to full URLs
	base, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	for i, ref := range urls {
		urlRef, err := url.Parse(ref)
		if err != nil {
			return nil, err
		}
		u := base.ResolveReference(urlRef)
		urls[i] = u.String()
	}

	// Attempt fetching files from each link
	chFiles := make(chan pdf)
	chErrors := make(chan error)
	var wg sync.WaitGroup

	for _, ref := range urls {
		wg.Add(1)
		go func(url string) {
			downloadFile(client, url, chFiles, chErrors)
			wg.Done()
		}(ref)
	}

	for i := 0; i < len(urls); i++ {
		select {
		case file := <-chFiles:
			files = append(files, file)
		case err := <-chErrors:
			return nil, err
		}
	}
	wg.Wait()
	return files, nil
}

func createCustomField(ctx context.Context, client *http.Client, name string) (int, error) {
	type CustomField struct {
		Data struct {
			DisplayOrder    int    `json:"display_order,omitempty"`
			Displayed       string `json:"displayed,omitempty"`
			FieldType       string `json:"field_type,omitempty"`
			Name            string `json:"name,omitempty"`
			ParentType      string `json:"parent_type,omitempty"`
			PicklistOptions []struct {
				ID      int    `json:"id,omitempty"`
				Option  string `json:"option,omitempty"`
				Deleted bool   `json:"_deleted,omitempty"`
			} `json:"picklist_options,omitempty"`
			Required string `json:"required,omitempty"`
		} `json:"data,omitempty"`
	}

	type CustomFieldResponse struct {
		Data struct {
			ID              int       `json:"id"`
			Etag            string    `json:"etag"`
			CreatedAt       time.Time `json:"created_at"`
			UpdatedAt       time.Time `json:"updated_at"`
			Name            string    `json:"name"`
			ParentType      string    `json:"parent_type"`
			FieldType       string    `json:"field_type"`
			Displayed       bool      `json:"displayed"`
			Deleted         bool      `json:"deleted"`
			Required        bool      `json:"required"`
			DisplayOrder    string    `json:"display_order"`
			PicklistOptions []struct {
				ID        int       `json:"id"`
				Option    string    `json:"option"`
				DeletedAt time.Time `json:"deleted_at"`
			} `json:"picklist_options"`
		} `json:"data"`
	}

	var customField CustomField
	customField.Data.Name = name
	customField.Data.ParentType = "Matter"
	customField.Data.FieldType = "text_line"

	payload, err := json.Marshal(customField)
	if err != nil {
		return 0, err
	}

	u := "https://app.clio.com/api/v4/custom_fields.json"
	req, err := http.NewRequest("POST", u, bytes.NewBuffer(payload))
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != 201 {
		return 0, errors.New("Could not create new field")
	}

	body, err := ioutil.ReadAll(resp.Body)

	var cfr CustomFieldResponse
	err = json.Unmarshal(body, &cfr)

	return cfr.Data.ID, nil
}

func downloadFile(client *http.Client, u string, ch chan pdf, chErrors chan error) {

	resp, err := client.Get(u)
	if err != nil {
		chErrors <- err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") == "application/pdf" {
		disposition := strings.SplitAfter(resp.Header.Get("Content-Disposition"), "filename=")
		name := strings.Trim(disposition[1], "\"")
		file, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			chErrors <- err
		}
		ch <- pdf{Name: name, Content: file, URL: u}
	} else {
		chErrors <- errors.New("Invalid or stale link")
	}
}

func getCustomField(ctx context.Context, client *http.Client, uid string, name string) (int, error) {
	type CustomFields struct {
		Data []struct {
			ID              int       `json:"id"`
			Etag            string    `json:"etag"`
			CreatedAt       time.Time `json:"created_at"`
			UpdatedAt       time.Time `json:"updated_at"`
			Name            string    `json:"name"`
			ParentType      string    `json:"parent_type"`
			FieldType       string    `json:"field_type"`
			Displayed       bool      `json:"displayed"`
			Deleted         bool      `json:"deleted"`
			Required        bool      `json:"required"`
			DisplayOrder    string    `json:"display_order"`
			PicklistOptions []struct {
				ID        int       `json:"id"`
				Option    string    `json:"option"`
				DeletedAt time.Time `json:"deleted_at"`
			} `json:"picklist_options"`
		} `json:"data"`
	}

	cache := pool.Get()
	defer cache.Close()

	odysseyID, err := redis.Int(cache.Do("GET", uid+":customField:Odyssey"))
	if err != nil && err != redis.ErrNil {
		return 0, err
	}

	// "Odyssey" custom field ID in cache
	if err != redis.ErrNil {
		return odysseyID, nil
	}

	// Retrieve "Odyssey" custom field ID from Clio API
	params := url.Values{}
	params.Add("query", "Odyssey")
	u := "https://app.clio.com/api/v4/custom_fields.json"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.URL.RawQuery = params.Encode()
	req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != 200 {
		return 0, errors.New("Unable to fetch ID for custom field")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, nil
	}

	var customFields CustomFields
	err = json.Unmarshal(body, &customFields)
	if err != nil {
		return 0, err
	}

	if len(customFields.Data) == 0 {
		return createCustomField(ctx, client, "Odyssey")
	}

	return customFields.Data[0].ID, nil

}

func getMatterID(ctx context.Context, client *http.Client, caseNumber string, uid string) (int, error) {

	type Matter struct {
		Data []struct {
			ID              int       `json:"id"`
			Etag            string    `json:"etag"`
			Number          int       `json:"number"`
			DisplayNumber   string    `json:"display_number"`
			CustomNumber    string    `json:"custom_number"`
			Description     string    `json:"description"`
			Status          string    `json:"status"`
			Location        string    `json:"location"`
			ClientReference string    `json:"client_reference"`
			Billable        bool      `json:"billable"`
			MaildropAddress string    `json:"maildrop_address"`
			BillingMethod   string    `json:"billing_method"`
			OpenDate        string    `json:"open_date"`
			CloseDate       string    `json:"close_date"`
			PendingDate     string    `json:"pending_date"`
			CreatedAt       time.Time `json:"created_at"`
			UpdatedAt       time.Time `json:"updated_at"`
			Shared          bool      `json:"shared"`
			Client          []struct {
				ID                     int       `json:"id"`
				Etag                   string    `json:"etag"`
				Name                   string    `json:"name"`
				FirstName              string    `json:"first_name"`
				MiddleName             string    `json:"middle_name"`
				LastName               string    `json:"last_name"`
				Type                   string    `json:"type"`
				CreatedAt              time.Time `json:"created_at"`
				UpdatedAt              time.Time `json:"updated_at"`
				Prefix                 string    `json:"prefix"`
				Title                  string    `json:"title"`
				Initials               string    `json:"initials"`
				ClioConnectEmail       string    `json:"clio_connect_email"`
				LockedClioConnectEmail bool      `json:"locked_clio_connect_email"`
				ClientConnectUserID    int       `json:"client_connect_user_id"`
				PrimaryEmailAddress    string    `json:"primary_email_address"`
				PrimaryPhoneNumber     string    `json:"primary_phone_number"`
				LedesClientID          string    `json:"ledes_client_id"`
				IsClient               bool      `json:"is_client"`
			} `json:"client"`
			ContingencyFee []struct {
				ID                   int    `json:"id"`
				Etag                 string `json:"etag"`
				ShowContingencyAward bool   `json:"show_contingency_award"`
			} `json:"contingency_fee"`
			CustomRate []struct {
				Type      string `json:"type"`
				OnInvoice bool   `json:"on_invoice"`
				Rates     []struct {
					ID    int    `json:"id"`
					Rate  int    `json:"rate"`
					Award int    `json:"award"`
					Note  string `json:"note"`
					Date  string `json:"date"`
					User  []struct {
						Enabled bool   `json:"enabled"`
						Etag    string `json:"etag"`
						ID      int    `json:"id"`
						Name    string `json:"name"`
					} `json:"user"`
					Group []struct {
						ID   int    `json:"id"`
						Etag string `json:"etag"`
						Name string `json:"name"`
					} `json:"group"`
					ActivityDescription []struct {
						ID   int    `json:"id"`
						Etag string `json:"etag"`
						Name string `json:"name"`
					} `json:"activity_description"`
				} `json:"rates"`
			} `json:"custom_rate"`
			Folder []struct {
				ID        int       `json:"id"`
				Etag      string    `json:"etag"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				DeletedAt time.Time `json:"deleted_at"`
				Type      string    `json:"type"`
				Locked    bool      `json:"locked"`
				Name      string    `json:"name"`
				Root      bool      `json:"root"`
			} `json:"folder"`
			Group []struct {
				ID                int       `json:"id"`
				Etag              string    `json:"etag"`
				Name              string    `json:"name"`
				Type              string    `json:"type"`
				CreatedAt         time.Time `json:"created_at"`
				UpdatedAt         time.Time `json:"updated_at"`
				ClientConnectUser bool      `json:"client_connect_user"`
			} `json:"group"`
			MatterBudget []struct {
				ID                    int       `json:"id"`
				Etag                  string    `json:"etag"`
				Budget                int       `json:"budget"`
				IncludeExpenses       bool      `json:"include_expenses"`
				NotificationThreshold int       `json:"notification_threshold"`
				NotifyUsers           bool      `json:"notify_users"`
				CreatedAt             time.Time `json:"created_at"`
				UpdatedAt             time.Time `json:"updated_at"`
			} `json:"matter_budget"`
			OriginatingAttorney []struct {
				AccountOwner              bool      `json:"account_owner"`
				ClioConnect               bool      `json:"clio_connect"`
				CourtRulesDefaultAttendee bool      `json:"court_rules_default_attendee"`
				DefaultCalendarID         int       `json:"default_calendar_id"`
				Email                     string    `json:"email"`
				Enabled                   bool      `json:"enabled"`
				Etag                      string    `json:"etag"`
				FirstName                 string    `json:"first_name"`
				ID                        int       `json:"id"`
				Initials                  string    `json:"initials"`
				LastName                  string    `json:"last_name"`
				Name                      string    `json:"name"`
				PhoneNumber               string    `json:"phone_number"`
				Rate                      int       `json:"rate"`
				SubscriptionType          string    `json:"subscription_type"`
				TimeZone                  string    `json:"time_zone"`
				Roles                     []string  `json:"roles"`
				CreatedAt                 time.Time `json:"created_at"`
				UpdatedAt                 time.Time `json:"updated_at"`
			} `json:"originating_attorney"`
			PracticeArea []struct {
				ID        int       `json:"id"`
				Etag      string    `json:"etag"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Name      string    `json:"name"`
				Code      string    `json:"code"`
			} `json:"practice_area"`
			ResponsibleAttorney []struct {
				AccountOwner              bool      `json:"account_owner"`
				ClioConnect               bool      `json:"clio_connect"`
				CourtRulesDefaultAttendee bool      `json:"court_rules_default_attendee"`
				DefaultCalendarID         int       `json:"default_calendar_id"`
				Email                     string    `json:"email"`
				Enabled                   bool      `json:"enabled"`
				Etag                      string    `json:"etag"`
				FirstName                 string    `json:"first_name"`
				ID                        int       `json:"id"`
				Initials                  string    `json:"initials"`
				LastName                  string    `json:"last_name"`
				Name                      string    `json:"name"`
				PhoneNumber               string    `json:"phone_number"`
				Rate                      int       `json:"rate"`
				SubscriptionType          string    `json:"subscription_type"`
				TimeZone                  string    `json:"time_zone"`
				Roles                     []string  `json:"roles"`
				CreatedAt                 time.Time `json:"created_at"`
				UpdatedAt                 time.Time `json:"updated_at"`
			} `json:"responsible_attorney"`
			StatuteOfLimitations []struct {
				ID                   int       `json:"id"`
				Etag                 string    `json:"etag"`
				Name                 string    `json:"name"`
				Status               string    `json:"status"`
				Description          string    `json:"description"`
				Priority             string    `json:"priority"`
				DueAt                string    `json:"due_at"`
				CompletedAt          time.Time `json:"completed_at"`
				NotifyCompletion     bool      `json:"notify_completion"`
				StatuteOfLimitations bool      `json:"statute_of_limitations"`
				CreatedAt            time.Time `json:"created_at"`
				UpdatedAt            time.Time `json:"updated_at"`
			} `json:"statute_of_limitations"`
			User []struct {
				AccountOwner              bool      `json:"account_owner"`
				ClioConnect               bool      `json:"clio_connect"`
				CourtRulesDefaultAttendee bool      `json:"court_rules_default_attendee"`
				DefaultCalendarID         int       `json:"default_calendar_id"`
				Email                     string    `json:"email"`
				Enabled                   bool      `json:"enabled"`
				Etag                      string    `json:"etag"`
				FirstName                 string    `json:"first_name"`
				ID                        int       `json:"id"`
				Initials                  string    `json:"initials"`
				LastName                  string    `json:"last_name"`
				Name                      string    `json:"name"`
				PhoneNumber               string    `json:"phone_number"`
				Rate                      int       `json:"rate"`
				SubscriptionType          string    `json:"subscription_type"`
				TimeZone                  string    `json:"time_zone"`
				Roles                     []string  `json:"roles"`
				CreatedAt                 time.Time `json:"created_at"`
				UpdatedAt                 time.Time `json:"updated_at"`
			} `json:"user"`
			Import []struct {
				ID                 int       `json:"id"`
				Etag               string    `json:"etag"`
				Status             string    `json:"status"`
				Source             string    `json:"source"`
				ImporterType       string    `json:"importer_type"`
				FileType           string    `json:"file_type"`
				HasMappings        bool      `json:"has_mappings"`
				UpdatedAt          time.Time `json:"updated_at"`
				CreatedAt          time.Time `json:"created_at"`
				ByteOffset         int       `json:"byte_offset"`
				FileSize           int       `json:"file_size"`
				ErrorFileAvailable bool      `json:"error_file_available"`
				RecordCount        int       `json:"record_count"`
				ErrorCount         int       `json:"error_count"`
				FatalErrorMessage  string    `json:"fatal_error_message"`
				Undone             bool      `json:"undone"`
				Stopped            bool      `json:"stopped"`
				OriginalFilename   string    `json:"original_filename"`
			} `json:"import"`
			AccountBalances []struct {
				ID      int    `json:"id"`
				Balance int    `json:"balance"`
				Type    string `json:"type"`
				Name    string `json:"name"`
			} `json:"account_balances"`
			CustomFieldValues []struct {
				ID                string    `json:"id"`
				Etag              string    `json:"etag"`
				FieldName         string    `json:"field_name"`
				CreatedAt         time.Time `json:"created_at"`
				UpdatedAt         time.Time `json:"updated_at"`
				FieldType         string    `json:"field_type"`
				FieldRequired     bool      `json:"field_required"`
				FieldDisplayed    bool      `json:"field_displayed"`
				FieldDisplayOrder int       `json:"field_display_order"`
				Value             string    `json:"value"`
				SoftDeleted       bool      `json:"soft_deleted"`
			} `json:"custom_field_values"`
			CustomFieldSetAssociations []struct {
				ID           int       `json:"id"`
				Etag         string    `json:"etag"`
				DisplayOrder int       `json:"display_order"`
				CreatedAt    time.Time `json:"created_at"`
				UpdatedAt    time.Time `json:"updated_at"`
			} `json:"custom_field_set_associations"`
			Relationships []struct {
				ID          int       `json:"id"`
				Etag        string    `json:"etag"`
				Description string    `json:"description"`
				CreatedAt   time.Time `json:"created_at"`
				UpdatedAt   time.Time `json:"updated_at"`
			} `json:"relationships"`
			TaskTemplateListInstances []struct {
				ID        int       `json:"id"`
				Etag      string    `json:"etag"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
			} `json:"task_template_list_instances"`
		} `json:"data"`
	}

	odysseyID, err := getCustomField(ctx, client, uid, "Odyssey")
	if err != nil {
		return 0, err
	}

	log.Println(odysseyID)

	var params url.Values
	params.Add("custom_field_values["+strconv.Itoa(odysseyID)+"]", caseNumber)
	u := "https://app.clio.com/api/v4/matters.json"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.URL.RawQuery = params.Encode()
	req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != 200 {
		return 0, errors.New("Error contacting clio for matter ID")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var matter Matter
	err = json.Unmarshal(body, &matter)
	if err != nil {
		return 0, err
	}

	if len(matter.Data) == 0 {
		return 0, errors.New("No matter associated")
	}

	return matter.Data[0].ID, nil
}

func uploadFile(client *http.Client, matterID int, file pdf, ch chan error) {
	type Inquiry struct {
		Data struct {
			Name   string `json:"name"`
			Parent struct {
				ID   int    `json:"id"`
				Type string `json:"type"`
			} `json:"parent"`
		} `json:"data"`
	}

	type Bucket struct {
		Data struct {
			ID                    int `json:"id"`
			LatestDocumentVersion struct {
				UUID       string `json:"uuid"`
				PutURL     string `json:"put_url"`
				PutHeaders []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"put_headers"`
			} `json:"latest_document_version"`
		} `json:"data"`
	}

	type Uploaded struct {
		Data struct {
			UUID          string `json:"uuid"`
			FullyUploaded string `json:"fully_uploaded"`
		} `json:"data"`
	}
	// Get bucket
	var inq Inquiry
	inq.Data.Name = file.Name
	inq.Data.Parent.ID = matterID
	inq.Data.Parent.Type = "Matter"
	jsonInq, err := json.Marshal(inq)
	if err != nil {
		ch <- err
		return
	}

	params := url.Values{}
	params.Add("fields", "id,latest_document_version{uuid,put_url,put_headers}")
	u := "https://app.clio.com/api/v4/documents.json"

	post, err := http.NewRequest("POST", u, bytes.NewBuffer(jsonInq))
	post.URL.RawQuery = params.Encode()
	post.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(post)
	if err != nil {
		ch <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		ch <- errors.New(resp.Status)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	var bucket Bucket
	err = json.Unmarshal(body, &bucket)
	if err != nil {
		ch <- err
		return
	}

	// Upload File

	put, err := http.NewRequest("PUT", bucket.Data.LatestDocumentVersion.PutURL, bytes.NewBuffer(file.Content))
	for _, header := range bucket.Data.LatestDocumentVersion.PutHeaders {
		put.Header.Add(header.Name, header.Value)
	}

	newClient := &http.Client{}
	resp2, err := newClient.Do(put)
	if err != nil {
		log.Println(err.Error())
		ch <- err
	}
	defer resp2.Body.Close()
	body, err = ioutil.ReadAll(resp2.Body)

	if resp2.StatusCode != 200 {
		ch <- errors.New(resp2.Status)
		return
	}

	// Mark completed
	var uploaded Uploaded
	uploaded.Data.UUID = bucket.Data.LatestDocumentVersion.UUID
	uploaded.Data.FullyUploaded = "true"
	jsonUploaded, err := json.Marshal(uploaded)

	params = url.Values{}
	params.Add("fields", "id,latest_document_version{fully_uploaded}")
	params.Add("external_property_name", "link")
	params.Add("external_property_value", file.URL)
	u = "https://app.clio.com/api/v4/documents/" + strconv.Itoa(bucket.Data.ID)

	patch, err := http.NewRequest("PATCH", u, bytes.NewBuffer(jsonUploaded))
	patch.Header.Add("Content-Type", "application/json")
	patch.URL.RawQuery = params.Encode()

	resp3, err := client.Do(patch)
	if err != nil {
		ch <- err
		return
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		ch <- errors.New(resp3.Status)
		return
	}

	return
}
