package v1alpha1

import (
	"testing"

	"errors"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
)

func TestStorageControlPlane(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}
	created := &ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		}}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &ControlPlane{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}

func TestGetNetwork(t *testing.T) {
	validNetwork := &Network{
		VpcCidr: "172.16.0.0/16",
		SubnetCidrs: []string{
			"172.16.0.0/24",
			"172.16.1.0/24",
			"172.16.2.0/24",
		},
	}
	tests := []struct {
		name string
		cp   *ControlPlane
		want Network
		err  error
	}{
		{
			name: "Network is nil",
			cp: &ControlPlane{
				Spec: ControlPlaneSpec{ClusterName: "test"},
			},
			want: DefaultNetwork,
			err:  nil,
		},
		{
			name: "Network is Invalid",
			cp: &ControlPlane{
				Spec: ControlPlaneSpec{
					ClusterName: "test",
					Network: &Network{
						VpcCidr: "172.16.0.0/16",
						SubnetCidrs: []string{
							"172.16.0.0/23",
							"172.16.1.0/24",
							"172.16.2.0/24",
						},
					},
				},
			},
			want: Network{},
			err:  errors.New("invalid network: 172.16.1.0/24 overlaps with 172.16.0.0/23"),
		},
		{
			name: "Network is valid",
			cp: &ControlPlane{
				Spec: ControlPlaneSpec{
					ClusterName: "test",
					Network:     validNetwork,
				},
			},
			want: *validNetwork,
			err:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cp.GetNetwork()
			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("Expected: \n%+v\n\n Got:\n%+v", tc.want, got)
			}

			if err != nil && tc.err.Error() != err.Error() {
				t.Errorf("Expected Error: \n%+v\n\n Got:\n%+v", tc.err, err)
			}
		})
	}
}
