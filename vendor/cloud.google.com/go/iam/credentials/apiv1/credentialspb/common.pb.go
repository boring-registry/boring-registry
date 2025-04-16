// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v4.25.3
// source: google/iam/credentials/v1/common.proto

package credentialspb

import (
	reflect "reflect"
	sync "sync"

	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GenerateAccessTokenRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required. The resource name of the service account for which the credentials
	// are requested, in the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The sequence of service accounts in a delegation chain. Each service
	// account must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on its next service account in the chain. The last service account in the
	// chain must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on the service account that is specified in the `name` field of the
	// request.
	//
	// The delegates must have the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Delegates []string `protobuf:"bytes,2,rep,name=delegates,proto3" json:"delegates,omitempty"`
	// Required. Code to identify the scopes to be included in the OAuth 2.0 access token.
	// See https://developers.google.com/identity/protocols/googlescopes for more
	// information.
	// At least one value required.
	Scope []string `protobuf:"bytes,4,rep,name=scope,proto3" json:"scope,omitempty"`
	// The desired lifetime duration of the access token in seconds.
	// Must be set to a value less than or equal to 3600 (1 hour). If a value is
	// not specified, the token's lifetime will be set to a default value of one
	// hour.
	Lifetime *durationpb.Duration `protobuf:"bytes,7,opt,name=lifetime,proto3" json:"lifetime,omitempty"`
}

func (x *GenerateAccessTokenRequest) Reset() {
	*x = GenerateAccessTokenRequest{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GenerateAccessTokenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateAccessTokenRequest) ProtoMessage() {}

func (x *GenerateAccessTokenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateAccessTokenRequest.ProtoReflect.Descriptor instead.
func (*GenerateAccessTokenRequest) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{0}
}

func (x *GenerateAccessTokenRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *GenerateAccessTokenRequest) GetDelegates() []string {
	if x != nil {
		return x.Delegates
	}
	return nil
}

func (x *GenerateAccessTokenRequest) GetScope() []string {
	if x != nil {
		return x.Scope
	}
	return nil
}

func (x *GenerateAccessTokenRequest) GetLifetime() *durationpb.Duration {
	if x != nil {
		return x.Lifetime
	}
	return nil
}

type GenerateAccessTokenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The OAuth 2.0 access token.
	AccessToken string `protobuf:"bytes,1,opt,name=access_token,json=accessToken,proto3" json:"access_token,omitempty"`
	// Token expiration time.
	// The expiration time is always set.
	ExpireTime *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=expire_time,json=expireTime,proto3" json:"expire_time,omitempty"`
}

func (x *GenerateAccessTokenResponse) Reset() {
	*x = GenerateAccessTokenResponse{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GenerateAccessTokenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateAccessTokenResponse) ProtoMessage() {}

func (x *GenerateAccessTokenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateAccessTokenResponse.ProtoReflect.Descriptor instead.
func (*GenerateAccessTokenResponse) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{1}
}

func (x *GenerateAccessTokenResponse) GetAccessToken() string {
	if x != nil {
		return x.AccessToken
	}
	return ""
}

func (x *GenerateAccessTokenResponse) GetExpireTime() *timestamppb.Timestamp {
	if x != nil {
		return x.ExpireTime
	}
	return nil
}

type SignBlobRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required. The resource name of the service account for which the credentials
	// are requested, in the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The sequence of service accounts in a delegation chain. Each service
	// account must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on its next service account in the chain. The last service account in the
	// chain must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on the service account that is specified in the `name` field of the
	// request.
	//
	// The delegates must have the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Delegates []string `protobuf:"bytes,3,rep,name=delegates,proto3" json:"delegates,omitempty"`
	// Required. The bytes to sign.
	Payload []byte `protobuf:"bytes,5,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (x *SignBlobRequest) Reset() {
	*x = SignBlobRequest{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignBlobRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignBlobRequest) ProtoMessage() {}

func (x *SignBlobRequest) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignBlobRequest.ProtoReflect.Descriptor instead.
func (*SignBlobRequest) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{2}
}

func (x *SignBlobRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *SignBlobRequest) GetDelegates() []string {
	if x != nil {
		return x.Delegates
	}
	return nil
}

func (x *SignBlobRequest) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

type SignBlobResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The ID of the key used to sign the blob.
	KeyId string `protobuf:"bytes,1,opt,name=key_id,json=keyId,proto3" json:"key_id,omitempty"`
	// The signed blob.
	SignedBlob []byte `protobuf:"bytes,4,opt,name=signed_blob,json=signedBlob,proto3" json:"signed_blob,omitempty"`
}

func (x *SignBlobResponse) Reset() {
	*x = SignBlobResponse{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignBlobResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignBlobResponse) ProtoMessage() {}

func (x *SignBlobResponse) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignBlobResponse.ProtoReflect.Descriptor instead.
func (*SignBlobResponse) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{3}
}

func (x *SignBlobResponse) GetKeyId() string {
	if x != nil {
		return x.KeyId
	}
	return ""
}

func (x *SignBlobResponse) GetSignedBlob() []byte {
	if x != nil {
		return x.SignedBlob
	}
	return nil
}

type SignJwtRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required. The resource name of the service account for which the credentials
	// are requested, in the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The sequence of service accounts in a delegation chain. Each service
	// account must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on its next service account in the chain. The last service account in the
	// chain must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on the service account that is specified in the `name` field of the
	// request.
	//
	// The delegates must have the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Delegates []string `protobuf:"bytes,3,rep,name=delegates,proto3" json:"delegates,omitempty"`
	// Required. The JWT payload to sign: a JSON object that contains a JWT Claims Set.
	Payload string `protobuf:"bytes,5,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (x *SignJwtRequest) Reset() {
	*x = SignJwtRequest{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignJwtRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignJwtRequest) ProtoMessage() {}

func (x *SignJwtRequest) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignJwtRequest.ProtoReflect.Descriptor instead.
func (*SignJwtRequest) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{4}
}

func (x *SignJwtRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *SignJwtRequest) GetDelegates() []string {
	if x != nil {
		return x.Delegates
	}
	return nil
}

func (x *SignJwtRequest) GetPayload() string {
	if x != nil {
		return x.Payload
	}
	return ""
}

type SignJwtResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The ID of the key used to sign the JWT.
	KeyId string `protobuf:"bytes,1,opt,name=key_id,json=keyId,proto3" json:"key_id,omitempty"`
	// The signed JWT.
	SignedJwt string `protobuf:"bytes,2,opt,name=signed_jwt,json=signedJwt,proto3" json:"signed_jwt,omitempty"`
}

func (x *SignJwtResponse) Reset() {
	*x = SignJwtResponse{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignJwtResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignJwtResponse) ProtoMessage() {}

func (x *SignJwtResponse) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignJwtResponse.ProtoReflect.Descriptor instead.
func (*SignJwtResponse) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{5}
}

func (x *SignJwtResponse) GetKeyId() string {
	if x != nil {
		return x.KeyId
	}
	return ""
}

func (x *SignJwtResponse) GetSignedJwt() string {
	if x != nil {
		return x.SignedJwt
	}
	return ""
}

type GenerateIdTokenRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required. The resource name of the service account for which the credentials
	// are requested, in the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The sequence of service accounts in a delegation chain. Each service
	// account must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on its next service account in the chain. The last service account in the
	// chain must be granted the `roles/iam.serviceAccountTokenCreator` role
	// on the service account that is specified in the `name` field of the
	// request.
	//
	// The delegates must have the following format:
	// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`. The `-` wildcard
	// character is required; replacing it with a project ID is invalid.
	Delegates []string `protobuf:"bytes,2,rep,name=delegates,proto3" json:"delegates,omitempty"`
	// Required. The audience for the token, such as the API or account that this token
	// grants access to.
	Audience string `protobuf:"bytes,3,opt,name=audience,proto3" json:"audience,omitempty"`
	// Include the service account email in the token. If set to `true`, the
	// token will contain `email` and `email_verified` claims.
	IncludeEmail bool `protobuf:"varint,4,opt,name=include_email,json=includeEmail,proto3" json:"include_email,omitempty"`
}

func (x *GenerateIdTokenRequest) Reset() {
	*x = GenerateIdTokenRequest{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GenerateIdTokenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateIdTokenRequest) ProtoMessage() {}

func (x *GenerateIdTokenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateIdTokenRequest.ProtoReflect.Descriptor instead.
func (*GenerateIdTokenRequest) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{6}
}

func (x *GenerateIdTokenRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *GenerateIdTokenRequest) GetDelegates() []string {
	if x != nil {
		return x.Delegates
	}
	return nil
}

func (x *GenerateIdTokenRequest) GetAudience() string {
	if x != nil {
		return x.Audience
	}
	return ""
}

func (x *GenerateIdTokenRequest) GetIncludeEmail() bool {
	if x != nil {
		return x.IncludeEmail
	}
	return false
}

type GenerateIdTokenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The OpenId Connect ID token.
	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *GenerateIdTokenResponse) Reset() {
	*x = GenerateIdTokenResponse{}
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GenerateIdTokenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateIdTokenResponse) ProtoMessage() {}

func (x *GenerateIdTokenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_google_iam_credentials_v1_common_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateIdTokenResponse.ProtoReflect.Descriptor instead.
func (*GenerateIdTokenResponse) Descriptor() ([]byte, []int) {
	return file_google_iam_credentials_v1_common_proto_rawDescGZIP(), []int{7}
}

func (x *GenerateIdTokenResponse) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

var File_google_iam_credentials_v1_common_proto protoreflect.FileDescriptor

var file_google_iam_credentials_v1_common_proto_rawDesc = []byte{
	0x0a, 0x26, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x69, 0x61, 0x6d, 0x2f, 0x63, 0x72, 0x65,
	0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2f, 0x76, 0x31, 0x2f, 0x63, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x69, 0x61, 0x6d, 0x2e, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73,
	0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f,
	0x66, 0x69, 0x65, 0x6c, 0x64, 0x5f, 0x62, 0x65, 0x68, 0x61, 0x76, 0x69, 0x6f, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69,
	0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0xcb, 0x01, 0x0a, 0x1a, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x41, 0x63, 0x63,
	0x65, 0x73, 0x73, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x3d, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x29, 0xe0,
	0x41, 0x02, 0xfa, 0x41, 0x23, 0x0a, 0x21, 0x69, 0x61, 0x6d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c,
	0x0a, 0x09, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x09, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x12, 0x19, 0x0a, 0x05,
	0x73, 0x63, 0x6f, 0x70, 0x65, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x42, 0x03, 0xe0, 0x41, 0x02,
	0x52, 0x05, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x12, 0x35, 0x0a, 0x08, 0x6c, 0x69, 0x66, 0x65, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x6c, 0x69, 0x66, 0x65, 0x74, 0x69, 0x6d, 0x65, 0x22, 0x7d,
	0x0a, 0x1b, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73,
	0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x21, 0x0a,
	0x0c, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x54, 0x6f, 0x6b, 0x65, 0x6e,
	0x12, 0x3b, 0x0a, 0x0b, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x52, 0x0a, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x22, 0x8d, 0x01,
	0x0a, 0x0f, 0x53, 0x69, 0x67, 0x6e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x3d, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x29, 0xe0, 0x41, 0x02, 0xfa, 0x41, 0x23, 0x0a, 0x21, 0x69, 0x61, 0x6d, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x1c, 0x0a, 0x09, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x09, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x12, 0x1d,
	0x0a, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x42,
	0x03, 0xe0, 0x41, 0x02, 0x52, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0x4a, 0x0a,
	0x10, 0x53, 0x69, 0x67, 0x6e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x15, 0x0a, 0x06, 0x6b, 0x65, 0x79, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x6b, 0x65, 0x79, 0x49, 0x64, 0x12, 0x1f, 0x0a, 0x0b, 0x73, 0x69, 0x67, 0x6e,
	0x65, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a, 0x73,
	0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x62, 0x22, 0x8c, 0x01, 0x0a, 0x0e, 0x53, 0x69,
	0x67, 0x6e, 0x4a, 0x77, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3d, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x29, 0xe0, 0x41, 0x02, 0xfa,
	0x41, 0x23, 0x0a, 0x21, 0x69, 0x61, 0x6d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x61, 0x70,
	0x69, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x41, 0x63,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x64,
	0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09,
	0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x12, 0x1d, 0x0a, 0x07, 0x70, 0x61, 0x79,
	0x6c, 0x6f, 0x61, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x42, 0x03, 0xe0, 0x41, 0x02, 0x52,
	0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0x47, 0x0a, 0x0f, 0x53, 0x69, 0x67, 0x6e,
	0x4a, 0x77, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x15, 0x0a, 0x06, 0x6b,
	0x65, 0x79, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6b, 0x65, 0x79,
	0x49, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x5f, 0x6a, 0x77, 0x74,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x4a, 0x77,
	0x74, 0x22, 0xbb, 0x01, 0x0a, 0x16, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x49, 0x64,
	0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3d, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x29, 0xe0, 0x41, 0x02, 0xfa,
	0x41, 0x23, 0x0a, 0x21, 0x69, 0x61, 0x6d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x61, 0x70,
	0x69, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x41, 0x63,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x64,
	0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09,
	0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x12, 0x1f, 0x0a, 0x08, 0x61, 0x75, 0x64,
	0x69, 0x65, 0x6e, 0x63, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x03, 0xe0, 0x41, 0x02,
	0x52, 0x08, 0x61, 0x75, 0x64, 0x69, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x69, 0x6e,
	0x63, 0x6c, 0x75, 0x64, 0x65, 0x5f, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x0c, 0x69, 0x6e, 0x63, 0x6c, 0x75, 0x64, 0x65, 0x45, 0x6d, 0x61, 0x69, 0x6c, 0x22,
	0x2f, 0x0a, 0x17, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x49, 0x64, 0x54, 0x6f, 0x6b,
	0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f,
	0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e,
	0x42, 0xac, 0x02, 0xea, 0x41, 0x59, 0x0a, 0x21, 0x69, 0x61, 0x6d, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x34, 0x70, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x73, 0x2f, 0x7b, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x7d, 0x2f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2f, 0x7b, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x7d, 0x0a,
	0x23, 0x63, 0x6f, 0x6d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x63, 0x6c, 0x6f, 0x75,
	0x64, 0x2e, 0x69, 0x61, 0x6d, 0x2e, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x73, 0x2e, 0x76, 0x31, 0x42, 0x19, 0x49, 0x41, 0x4d, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74,
	0x69, 0x61, 0x6c, 0x73, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50,
	0x01, 0x5a, 0x45, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x2f, 0x69, 0x61, 0x6d, 0x2f, 0x63, 0x72, 0x65, 0x64, 0x65,
	0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2f, 0x61, 0x70, 0x69, 0x76, 0x31, 0x2f, 0x63, 0x72, 0x65,
	0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x70, 0x62, 0x3b, 0x63, 0x72, 0x65, 0x64, 0x65,
	0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x70, 0x62, 0xf8, 0x01, 0x01, 0xaa, 0x02, 0x1f, 0x47, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x49, 0x61, 0x6d, 0x2e, 0x43,
	0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x1f,
	0x47, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x5c, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x5c, 0x49, 0x61, 0x6d,
	0x5c, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x5c, 0x56, 0x31, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_google_iam_credentials_v1_common_proto_rawDescOnce sync.Once
	file_google_iam_credentials_v1_common_proto_rawDescData = file_google_iam_credentials_v1_common_proto_rawDesc
)

func file_google_iam_credentials_v1_common_proto_rawDescGZIP() []byte {
	file_google_iam_credentials_v1_common_proto_rawDescOnce.Do(func() {
		file_google_iam_credentials_v1_common_proto_rawDescData = protoimpl.X.CompressGZIP(file_google_iam_credentials_v1_common_proto_rawDescData)
	})
	return file_google_iam_credentials_v1_common_proto_rawDescData
}

var file_google_iam_credentials_v1_common_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_google_iam_credentials_v1_common_proto_goTypes = []any{
	(*GenerateAccessTokenRequest)(nil),  // 0: google.iam.credentials.v1.GenerateAccessTokenRequest
	(*GenerateAccessTokenResponse)(nil), // 1: google.iam.credentials.v1.GenerateAccessTokenResponse
	(*SignBlobRequest)(nil),             // 2: google.iam.credentials.v1.SignBlobRequest
	(*SignBlobResponse)(nil),            // 3: google.iam.credentials.v1.SignBlobResponse
	(*SignJwtRequest)(nil),              // 4: google.iam.credentials.v1.SignJwtRequest
	(*SignJwtResponse)(nil),             // 5: google.iam.credentials.v1.SignJwtResponse
	(*GenerateIdTokenRequest)(nil),      // 6: google.iam.credentials.v1.GenerateIdTokenRequest
	(*GenerateIdTokenResponse)(nil),     // 7: google.iam.credentials.v1.GenerateIdTokenResponse
	(*durationpb.Duration)(nil),         // 8: google.protobuf.Duration
	(*timestamppb.Timestamp)(nil),       // 9: google.protobuf.Timestamp
}
var file_google_iam_credentials_v1_common_proto_depIdxs = []int32{
	8, // 0: google.iam.credentials.v1.GenerateAccessTokenRequest.lifetime:type_name -> google.protobuf.Duration
	9, // 1: google.iam.credentials.v1.GenerateAccessTokenResponse.expire_time:type_name -> google.protobuf.Timestamp
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_google_iam_credentials_v1_common_proto_init() }
func file_google_iam_credentials_v1_common_proto_init() {
	if File_google_iam_credentials_v1_common_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_google_iam_credentials_v1_common_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_google_iam_credentials_v1_common_proto_goTypes,
		DependencyIndexes: file_google_iam_credentials_v1_common_proto_depIdxs,
		MessageInfos:      file_google_iam_credentials_v1_common_proto_msgTypes,
	}.Build()
	File_google_iam_credentials_v1_common_proto = out.File
	file_google_iam_credentials_v1_common_proto_rawDesc = nil
	file_google_iam_credentials_v1_common_proto_goTypes = nil
	file_google_iam_credentials_v1_common_proto_depIdxs = nil
}
