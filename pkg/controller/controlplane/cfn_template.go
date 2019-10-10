package controlplane

var controlplaneCFNTemplate = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Networking + Control Plane [managed by aws-eks-cluster-controller]'

Parameters:
  EKSVersion:
    Type: String
    Description: EKS Version this control plane should run on
    Default: {{ .Version }}

Resources:
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: {{ .Network.VpcCidr }}
      EnableDnsSupport: true
      EnableDnsHostnames: true
      Tags:
      - Key: Name
        Value: !Sub '${AWS::StackName}-VPC'

  InternetGateway:
    Type: "AWS::EC2::InternetGateway"

  VPCGatewayAttachment:
    Type: "AWS::EC2::VPCGatewayAttachment"
    Properties:
      InternetGatewayId: !Ref InternetGateway
      VpcId: !Ref VPC

  RouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId: !Ref VPC
      Tags:
      - Key: Name
        Value: Public Subnets
      - Key: Network
        Value: Public

  Route:
    DependsOn: VPCGatewayAttachment
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref RouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway

  Subnet01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock: {{ index .Network.SubnetCidrs 0 }}
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet01"

  Subnet02:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 02
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock: {{ index .Network.SubnetCidrs 1 }}
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet02"

  Subnet03:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 03
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock: {{ index .Network.SubnetCidrs 2 }}
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet03"

  Subnet01RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet01
      RouteTableId: !Ref RouteTable

  Subnet02RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet02
      RouteTableId: !Ref RouteTable

  Subnet03RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet03
      RouteTableId: !Ref RouteTable

  ControlPlaneSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Cluster communication with worker nodes
      VpcId: !Ref VPC

  ServiceRole:
    Type: 'AWS::IAM::Role'
    Properties:
      AssumeRolePolicyDocument:
        Statement:
          - Action:
              - 'sts:AssumeRole'
            Effect: Allow
            Principal:
              Service:
                - eks.amazonaws.com
        Version: '2012-10-17'
      ManagedPolicyArns:
        - 'arn:aws:iam::aws:policy/AmazonEKSServicePolicy'
        - 'arn:aws:iam::aws:policy/AmazonEKSClusterPolicy'

  ControlPlane:
    Type: 'AWS::EKS::Cluster'
    Properties:
      Name: {{ .ClusterName }}
      ResourcesVpcConfig:
        SecurityGroupIds:
          - Ref: ControlPlaneSecurityGroup
        SubnetIds:
          - !Ref 'Subnet01'
          - !Ref 'Subnet02'
          - !Ref 'Subnet03'
      RoleArn:
        !GetAtt ServiceRole.Arn
      Version: !Ref EKSVersion
Outputs:

  SubnetIds:
    Export:
      Name: !Sub '${AWS::StackName}::SubnetIds'
    Value: !Join [ ",", [ !Ref Subnet01, !Ref Subnet02, !Ref Subnet03 ] ]

  SecurityGroup:
    Export:
      Name: !Sub '${AWS::StackName}::SecurityGroup'
    Value: !Join [ ",", [ !Ref ControlPlaneSecurityGroup ] ]

  VPC:
    Export:
      Name: !Sub '${AWS::StackName}::VPC'
    Value: !Ref VPC
  
  ClusterStackName:
    Export:
      Name: !Sub '${AWS::StackName}::ClusterStackName'
    Value: !Ref 'AWS::StackName'

  Endpoint:
    Export:
      Name: !Sub '${AWS::StackName}::Endpoint'
    Value:
      !GetAtt ControlPlane.Endpoint
`
